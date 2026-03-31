package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/YoeDistro/yoe-ng/internal/image"
	"github.com/YoeDistro/yoe-ng/internal/artifact"
	"github.com/YoeDistro/yoe-ng/internal/repo"
	"github.com/YoeDistro/yoe-ng/internal/resolve"
	"github.com/YoeDistro/yoe-ng/internal/source"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// Options controls build behavior.
// BuildEvent is sent to Options.OnEvent during a build.
type BuildEvent struct {
	Unit   string
	Status string // "cached", "building", "done", "failed"
}

type Options struct {
	Ctx        context.Context // optional; nil means background
	Force      bool   // rebuild even if cached
	Clean      bool   // delete build dir before rebuilding (implies Force)
	NoCache    bool   // skip all caches
	DryRun     bool   // show what would be built
	Verbose    bool   // show build output in console (default: log only)
	ProjectDir string // project root
	Arch       string // target architecture
	OnEvent    func(BuildEvent) // optional callback for build progress
}

// BuildUnits builds the specified units (or all if names is empty).
func BuildUnits(proj *yoestar.Project, names []string, opts Options, w io.Writer) error {
	dag, err := resolve.BuildDAG(proj)
	if err != nil {
		return err
	}

	// Determine build order
	order, err := dag.TopologicalSort()
	if err != nil {
		return err
	}

	// Compute hashes for cache
	hashes, err := resolve.ComputeAllHashes(dag, opts.Arch)
	if err != nil {
		return err
	}

	// Filter to requested units (and their deps)
	requested := make(map[string]bool)
	if len(names) > 0 {
		for _, n := range names {
			requested[n] = true
		}
		order, err = filterBuildOrder(dag, order, names)
		if err != nil {
			return err
		}
	}

	if opts.DryRun {
		return dryRun(w, proj, order, hashes, opts, requested)
	}

	notify := func(unit, status string) {
		if opts.OnEvent != nil {
			opts.OnEvent(BuildEvent{Unit: unit, Status: status})
		}
	}

	// Pre-scan: emit cached/waiting status for all units so the TUI
	// can show the full build queue before any work starts.
	for _, name := range order {
		hash := hashes[name]
		forceThis := (opts.Force || opts.Clean) && (len(requested) == 0 || requested[name])
		if !forceThis && !opts.NoCache && IsBuildCached(opts.ProjectDir, name, hash) {
			notify(name, "cached")
		} else {
			notify(name, "waiting")
		}
	}

	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Build in order
	for _, name := range order {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("build cancelled")
		}

		unit := proj.Units[name]
		hash := hashes[name]

		// --force/--clean only apply to explicitly requested units;
		// dependencies still use the cache.
		forceThis := (opts.Force || opts.Clean) && (len(requested) == 0 || requested[name])

		if !forceThis && !opts.NoCache {
			if IsBuildCached(opts.ProjectDir, name, hash) {
				fmt.Fprintf(w, "%-20s [cached] %s\n", name, hash[:12])
				continue
			}
		}

		fmt.Fprintf(w, "%-20s [building]\n", name)
		notify(name, "building")

		if err := buildOne(ctx, proj, dag, unit, hash, opts, w); err != nil {
			notify(name, "failed")
			// Show which remaining units are blocked by this failure
			blocked := blockedUnits(dag, name, order)
			if len(blocked) > 0 {
				fmt.Fprintf(w, "  the following units depend on %s and cannot be built:\n", name)
				for _, b := range blocked {
					fmt.Fprintf(w, "    - %s\n", b)
				}
			}
			return fmt.Errorf("building %s: %w", name, err)
		}

		// Write cache marker
		writeCacheMarker(opts.ProjectDir, name, hash)
		fmt.Fprintf(w, "%-20s [done] %s\n", name, hash[:12])
		notify(name, "done")
	}

	return nil
}

func buildOne(ctx context.Context, proj *yoestar.Project, dag *resolve.DAG, unit *yoestar.Unit, hash string, opts Options, w io.Writer) error {
	buildDir := UnitBuildDir(opts.ProjectDir, unit.Name)
	EnsureDir(buildDir)

	// Remove the cache marker before starting so a cancelled or failed
	// build does not leave a stale marker that makes it appear cached.
	os.Remove(CacheMarkerPath(opts.ProjectDir, unit.Name, hash))

	// Write a lock file so other yoe instances can detect an in-progress build.
	lockPath := BuildingLockPath(opts.ProjectDir, unit.Name)
	os.WriteFile(lockPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	defer os.Remove(lockPath)

	// Open build log. In verbose mode, tee to terminal + log file.
	// In normal mode, log only — on error, print the log path.
	logPath := filepath.Join(buildDir, "build.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("creating build log: %w", err)
	}
	defer logFile.Close()

	var logW io.Writer
	if opts.Verbose {
		logW = io.MultiWriter(w, logFile)
	} else {
		logW = logFile
	}

	// Image units go through a different path — assemble rootfs
	if unit.Class == "image" {
		outputDir := filepath.Join(buildDir, "output")
		if err := image.Assemble(unit, proj, opts.ProjectDir, outputDir, logW); err != nil {
			if !opts.Verbose {
				fmt.Fprintf(w, "  build log: %s\n", logPath)
			}
			return err
		}
		return nil
	}

	srcDir := filepath.Join(buildDir, "src")
	destDir := filepath.Join(buildDir, "destdir")

	if opts.Clean {
		os.RemoveAll(srcDir)
		os.RemoveAll(destDir)
	}

	// Always start with an empty destdir
	os.RemoveAll(destDir)
	EnsureDir(destDir)

	// Prepare source (fetch + extract + patch, or reuse dev source).
	// Units without a source field (e.g., musl) skip this step.
	if unit.Source != "" {
		if _, err := source.Prepare(opts.ProjectDir, unit, w); err != nil {
			return fmt.Errorf("preparing source: %w", err)
		}
	} else {
		EnsureDir(srcDir)
	}

	// Determine build commands based on class
	commands := buildCommands(unit)
	if len(commands) == 0 {
		fmt.Fprintf(w, "  (no build steps for %s class %q)\n", unit.Name, unit.Class)
		return nil
	}

	// Assemble per-unit sysroot from transitive deps
	sysroot := filepath.Join(buildDir, "sysroot")
	if err := AssembleSysroot(sysroot, dag, unit.Name, opts.ProjectDir); err != nil {
		return fmt.Errorf("assembling sysroot: %w", err)
	}
	env := map[string]string{
		"PREFIX":          "/usr",
		"DESTDIR":         "/build/destdir",
		"NPROC":           NProc(),
		"ARCH":            opts.Arch,
		"HOME":            "/tmp",
		"PATH":            "/build/sysroot/usr/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"PKG_CONFIG_PATH": "/build/sysroot/usr/lib/pkgconfig:/usr/lib/pkgconfig",
		"CFLAGS":          "-I/build/sysroot/usr/include",
		"CPPFLAGS":        "-I/build/sysroot/usr/include",
		"LDFLAGS":         "-L/build/sysroot/usr/lib",
		"LD_LIBRARY_PATH": "/build/sysroot/usr/lib",
		"PYTHONPATH":      "/build/sysroot/usr/lib/python3.12/site-packages",
	}

	// Execute each build step inside the container with bwrap
	for i, cmd := range commands {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("build cancelled")
		}
		fmt.Fprintf(logW, "  [%d/%d] %s\n", i+1, len(commands), cmd)

		cfg := &SandboxConfig{
			Ctx:        ctx,
			SrcDir:     srcDir,
			DestDir:    destDir,
			Sysroot:    sysroot,
			Env:        env,
			ProjectDir: opts.ProjectDir,
			Stdout:     logW,
			Stderr:     logW,
		}
		if err := RunInSandbox(cfg, cmd); err != nil {
			if !opts.Verbose {
				fmt.Fprintf(w, "  build log: %s\n", logPath)
			}
			return err
		}
	}

	// Package the output into an .apk and publish to the local repo
	if unit.Class != "image" {
		apkPath, err := artifact.CreateAPK(unit, destDir, filepath.Join(buildDir, "pkg"))
		if err != nil {
			return fmt.Errorf("creating apk: %w", err)
		}
		fmt.Fprintf(w, "  → %s\n", filepath.Base(apkPath))

		repoDir := repo.RepoDir(nil, opts.ProjectDir)
		if err := repo.Publish(apkPath, repoDir); err != nil {
			return fmt.Errorf("publishing to repo: %w", err)
		}

		// Stage destdir for downstream units' per-unit sysroots
		if err := StageSysroot(destDir, buildDir); err != nil {
			fmt.Fprintf(w, "  (warning: sysroot staging failed: %v)\n", err)
		}
	}

	return nil
}

// buildCommands returns the shell commands to execute for a unit.
func buildCommands(unit *yoestar.Unit) []string {
	// Explicit build steps take priority
	if len(unit.Build) > 0 {
		return unit.Build
	}

	// Class-specific defaults
	switch unit.Class {
	case "autotools":
		configureArgs := ""
		if len(unit.ConfigureArgs) > 0 {
			configureArgs = " " + joinArgs(unit.ConfigureArgs)
		}
		return []string{
			"./configure --prefix=$PREFIX" + configureArgs,
			"make -j$NPROC",
			"make DESTDIR=$DESTDIR install",
		}
	case "cmake":
		cmakeArgs := ""
		if len(unit.ConfigureArgs) > 0 {
			cmakeArgs = " " + joinArgs(unit.ConfigureArgs)
		}
		return []string{
			"cmake -B build -DCMAKE_INSTALL_PREFIX=$PREFIX" + cmakeArgs,
			"cmake --build build -j $NPROC",
			"DESTDIR=$DESTDIR cmake --install build",
		}
	case "go":
		pkg := unit.GoPackage
		if pkg == "" {
			pkg = "."
		}
		return []string{
			fmt.Sprintf("go build -o $DESTDIR/usr/bin/%s %s", unit.Name, pkg),
		}
	case "image":
		// Images are assembled, not compiled — handled separately
		return nil
	}

	return nil
}

func joinArgs(args []string) string {
	result := ""
	for _, a := range args {
		result += " " + a
	}
	return result
}

func filterBuildOrder(dag *resolve.DAG, fullOrder []string, names []string) ([]string, error) {
	needed := make(map[string]bool)
	for _, name := range names {
		if _, ok := dag.Nodes[name]; !ok {
			return nil, fmt.Errorf("unit %q not found", name)
		}
		needed[name] = true
		deps, _ := dag.DepsOf(name)
		for _, d := range deps {
			needed[d] = true
		}
	}

	var filtered []string
	for _, name := range fullOrder {
		if needed[name] {
			filtered = append(filtered, name)
		}
	}
	return filtered, nil
}

// blockedUnits returns units remaining in the build order that transitively
// depend on the failed unit.
func blockedUnits(dag *resolve.DAG, failed string, order []string) []string {
	rdeps, err := dag.RdepsOf(failed)
	if err != nil {
		return nil
	}
	rdepSet := make(map[string]bool, len(rdeps))
	for _, r := range rdeps {
		rdepSet[r] = true
	}
	// Return in build order for clarity
	var blocked []string
	for _, name := range order {
		if rdepSet[name] {
			blocked = append(blocked, name)
		}
	}
	return blocked
}

func dryRun(w io.Writer, proj *yoestar.Project, order []string, hashes map[string]string, opts Options, requested map[string]bool) error {
	fmt.Fprintln(w, "Dry run — would build in this order:")
	for _, name := range order {
		unit := proj.Units[name]
		cached := ""
		forceThis := (opts.Force || opts.Clean) && (len(requested) == 0 || requested[name])
		if !forceThis && IsBuildCached(opts.ProjectDir, name, hashes[name]) {
			cached = " [cached, skip]"
		}
		fmt.Fprintf(w, "  %-20s [%s] %s%s\n", name, unit.Class, hashes[name][:12], cached)
	}
	return nil
}

// --- Simple file-based cache ---

func CacheMarkerPath(projectDir, name, hash string) string {
	return filepath.Join(projectDir, "build", name, ".yoe-hash")
}

func IsBuildCached(projectDir, name, hash string) bool {
	data, err := os.ReadFile(CacheMarkerPath(projectDir, name, hash))
	if err != nil {
		return false
	}
	return string(data) == hash
}

func HasBuildLog(projectDir, name string) bool {
	_, err := os.Stat(filepath.Join(projectDir, "build", name, "build.log"))
	return err == nil
}

// BuildingLockPath returns the path of the lock file written during a build.
func BuildingLockPath(projectDir, name string) string {
	return filepath.Join(projectDir, "build", name, ".building")
}

// IsBuildInProgress returns true if another process is currently building this unit.
// It checks for the lock file and verifies the PID is still alive.
func IsBuildInProgress(projectDir, name string) bool {
	data, err := os.ReadFile(BuildingLockPath(projectDir, name))
	if err != nil {
		return false
	}
	pid := strings.TrimSpace(string(data))
	// Check if the process is still running
	_, err = os.Stat(fmt.Sprintf("/proc/%s", pid))
	return err == nil
}

func writeCacheMarker(projectDir, name, hash string) {
	path := CacheMarkerPath(projectDir, name, hash)
	EnsureDir(filepath.Dir(path))
	os.WriteFile(path, []byte(hash), 0644)
}
