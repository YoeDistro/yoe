package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoeDistro/yoe-ng/internal/artifact"
	"github.com/YoeDistro/yoe-ng/internal/repo"
	"github.com/YoeDistro/yoe-ng/internal/resolve"
	"github.com/YoeDistro/yoe-ng/internal/source"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
	"go.starlark.net/starlark"
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
	Machine    string // target machine name
	OnEvent    func(BuildEvent) // optional callback for build progress
}

// ScopeDir returns the build subdirectory for a unit based on its scope.
// "machine" → machine name, "noarch" → "noarch", default → arch.
func ScopeDir(unit *yoestar.Unit, arch, machine string) string {
	switch unit.Scope {
	case "machine":
		return machine
	case "noarch":
		return "noarch"
	default:
		return arch
	}
}

// BuildUnits builds the specified units (or all if names is empty).
func BuildUnits(proj *yoestar.Project, names []string, opts Options, w io.Writer) error {
	// Warn if old-style build directories exist (no arch subdirectory)


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
	hashes, err := resolve.ComputeAllHashes(dag, opts.Arch, opts.Machine)
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
		sd := ScopeDir(proj.Units[name], opts.Arch, opts.Machine)
		forceThis := (opts.Force || opts.Clean) && (len(requested) == 0 || requested[name])
		if !forceThis && !opts.NoCache && IsBuildCached(opts.ProjectDir, sd, name, hash) {
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
		sd := ScopeDir(unit, opts.Arch, opts.Machine)

		// --force/--clean only apply to explicitly requested units;
		// dependencies still use the cache.
		forceThis := (opts.Force || opts.Clean) && (len(requested) == 0 || requested[name])

		if !forceThis && !opts.NoCache {
			if IsBuildCached(opts.ProjectDir, sd, name, hash) {
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
		writeCacheMarker(opts.ProjectDir, sd, name, hash)
		fmt.Fprintf(w, "%-20s [done] %s\n", name, hash[:12])
		notify(name, "done")
	}

	return nil
}

func buildOne(ctx context.Context, proj *yoestar.Project, dag *resolve.DAG, unit *yoestar.Unit, hash string, opts Options, w io.Writer) (buildErr error) {
	sd := ScopeDir(unit, opts.Arch, opts.Machine)
	buildDir := UnitBuildDir(opts.ProjectDir, sd, unit.Name)
	EnsureDir(buildDir)

	// Skip if another process is already building this unit.
	if IsBuildInProgress(opts.ProjectDir, sd, unit.Name) {
		fmt.Fprintf(w, "  %s: build already in progress, skipping\n", unit.Name)
		return nil
	}

	// Remove the cache marker before starting so a cancelled or failed
	// build does not leave a stale marker that makes it appear cached.
	os.Remove(CacheMarkerPath(opts.ProjectDir, sd, unit.Name, hash))

	// Write a lock file so other yoe instances can detect an in-progress build.
	lockPath := BuildingLockPath(opts.ProjectDir, sd, unit.Name)
	os.WriteFile(lockPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	defer os.Remove(lockPath)

	// Write initial build metadata; update on completion.
	buildStart := time.Now()
	meta := &BuildMeta{
		Status:  "building",
		Started: &buildStart,
		Hash:    hash,
	}
	WriteMeta(buildDir, meta)
	defer func() {
		now := time.Now()
		meta.Finished = &now
		meta.Duration = now.Sub(buildStart).Seconds()
		meta.DiskBytes = DirSize(buildDir)
		meta.InstalledBytes = DirSize(filepath.Join(buildDir, "destdir"))
		if ctx.Err() != nil {
			meta.Status = "cancelled"
		} else if buildErr != nil {
			meta.Status = "failed"
			meta.Error = buildErr.Error()
		} else {
			meta.Status = "complete"
		}
		WriteMeta(buildDir, meta)
	}()

	// Write executor output to executor.log so TUI detail view can show it
	// even for CLI builds.
	outputPath := filepath.Join(buildDir, "executor.log")
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output log: %w", err)
	}
	defer outputFile.Close()
	w = io.MultiWriter(w, outputFile)

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
		if _, err := source.Prepare(opts.ProjectDir, sd, unit, w); err != nil {
			return fmt.Errorf("preparing source: %w", err)
		}
	} else {
		EnsureDir(srcDir)
	}

	if len(unit.Tasks) == 0 {
		fmt.Fprintf(w, "  (no tasks for %s class %q)\n", unit.Name, unit.Class)
		return nil
	}

	// Assemble per-unit sysroot from transitive deps
	sysroot := filepath.Join(buildDir, "sysroot")
	if err := AssembleSysroot(sysroot, dag, unit.Name, opts.ProjectDir, opts.Arch); err != nil {
		return fmt.Errorf("assembling sysroot: %w", err)
	}
	// Extract console device from machine kernel cmdline (e.g., "console=ttyS0,115200" → "ttyS0")
	console := ""
	if m, ok := proj.Machines[opts.Machine]; ok && m.Kernel.Cmdline != "" {
		for _, part := range strings.Split(m.Kernel.Cmdline, " ") {
			if strings.HasPrefix(part, "console=") {
				c := strings.TrimPrefix(part, "console=")
				if idx := strings.Index(c, ","); idx > 0 {
					c = c[:idx]
				}
				console = c
				break
			}
		}
	}

	env := map[string]string{
		"PREFIX":          "/usr",
		"DESTDIR":         "/build/destdir",
		"NPROC":           NProc(),
		"ARCH":            opts.Arch,
		"MACHINE":         opts.Machine,
		"CONSOLE":         console,
		"HOME":            "/tmp",
		"PATH":            "/build/sysroot/usr/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"PKG_CONFIG_PATH": "/build/sysroot/usr/lib/pkgconfig:/usr/lib/pkgconfig",
		"CFLAGS":          "-I/build/sysroot/usr/include",
		"CPPFLAGS":        "-I/build/sysroot/usr/include",
		"LDFLAGS":         "-L/build/sysroot/usr/lib",
		"LD_LIBRARY_PATH": "/build/sysroot/usr/lib",
		"PYTHONPATH":      "/build/sysroot/usr/lib/python3.12/site-packages",
		"REPO":            filepath.Join("/project", repoRelPath(proj, opts.ProjectDir)),
	}

	// Resolve container image for this unit
	containerImage := resolveContainerImage(proj, unit, opts.Arch)

	// For container units, set the host working directory to the .star file's
	// directory so docker build can find the Dockerfile.
	hostDir := ""
	if unit.Class == "container" && unit.DefinedIn != "" {
		hostDir = unit.DefinedIn
	}

	// Execute tasks
	for ti, t := range unit.Tasks {
		fmt.Fprintf(w, "  [%d/%d] task: %s\n", ti+1, len(unit.Tasks), t.Name)
		fmt.Fprintf(logW, "  task: %s (%d steps)\n", t.Name, len(t.Steps))

		for i, step := range t.Steps {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("build cancelled")
			}

			if step.Command != "" {
				fmt.Fprintf(logW, "    [%d/%d] %s\n", i+1, len(t.Steps), step.Command)
				cfg := &SandboxConfig{
					Ctx:        ctx,
					Arch:       opts.Arch,
					Container:  containerImage,
					SrcDir:     srcDir,
					DestDir:    destDir,
					Sysroot:    sysroot,
					Env:        env,
					ProjectDir: opts.ProjectDir,
					HostDir:    hostDir,
					Stdout:     logW,
					Stderr:     logW,
				}
				if err := RunInSandbox(cfg, step.Command); err != nil {
					if !opts.Verbose {
						fmt.Fprintf(w, "  build log: %s\n", logPath)
					}
					return err
				}
			} else if step.Fn != nil {
				fmt.Fprintf(logW, "    [%d/%d] fn: %s\n", i+1, len(t.Steps), step.Fn.Name())
				cfg := &SandboxConfig{
					Ctx:        ctx,
					Arch:       opts.Arch,
					Container:  containerImage,
					SrcDir:     srcDir,
					DestDir:    destDir,
					Sysroot:    sysroot,
					Env:        env,
					ProjectDir: opts.ProjectDir,
					HostDir:    hostDir,
					Stdout:     logW,
					Stderr:     logW,
				}
				thread := NewBuildThread(ctx, cfg, RealExecer{})
				if _, err := starlark.Call(thread, step.Fn, nil, nil); err != nil {
					if !opts.Verbose {
						fmt.Fprintf(w, "  build log: %s\n", logPath)
					}
					return fmt.Errorf("task %s: %w", t.Name, err)
				}
			}
		}
	}

	// Package the output into an .apk and publish to the local repo
	if unit.Class != "image" && unit.Class != "container" {
		apkPath, err := artifact.CreateAPK(unit, destDir, filepath.Join(buildDir, "pkg"), sd)
		if err != nil {
			return fmt.Errorf("creating apk: %w", err)
		}
		fmt.Fprintf(w, "  → %s\n", filepath.Base(apkPath))

		repoDir := repo.RepoDir(proj, opts.ProjectDir)
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
		sd := ScopeDir(unit, opts.Arch, opts.Machine)
		cached := ""
		forceThis := (opts.Force || opts.Clean) && (len(requested) == 0 || requested[name])
		if !forceThis && IsBuildCached(opts.ProjectDir, sd, name, hashes[name]) {
			cached = " [cached, skip]"
		}
		fmt.Fprintf(w, "  %-20s [%s] %s%s\n", name, unit.Class, hashes[name][:12], cached)
	}
	return nil
}

// resolveContainerImage returns the Docker image tag for a unit's container.
// For container units (referenced by name), the tag is yoe-ng/<name>:<version>-<arch>.
// For external images (containing ":" or "/"), the value is used directly.
func resolveContainerImage(proj *yoestar.Project, unit *yoestar.Unit, arch string) string {
	container := unit.Container
	if container == "" {
		return ""
	}

	// External image reference (e.g., "golang:1.23")
	if strings.Contains(container, ":") || strings.Contains(container, "/") {
		return container
	}

	// Container unit — look up version and build tag.
	// Always include arch in tag for explicitness.
	if cu, ok := proj.Units[container]; ok {
		imageArch := arch
		if unit.ContainerArch == "host" {
			imageArch = Arch()
		}
		return fmt.Sprintf("yoe/%s:%s-%s", container, cu.Version, imageArch)
	}

	return container
}

// --- Simple file-based cache ---

func CacheMarkerPath(projectDir, arch, name, hash string) string {
	return filepath.Join(UnitBuildDir(projectDir, arch, name), ".yoe-hash")
}

func IsBuildCached(projectDir, arch, name, hash string) bool {
	data, err := os.ReadFile(CacheMarkerPath(projectDir, arch, name, hash))
	if err != nil {
		return false
	}
	return string(data) == hash
}

func HasBuildLog(projectDir, arch, name string) bool {
	_, err := os.Stat(filepath.Join(UnitBuildDir(projectDir, arch, name), "build.log"))
	return err == nil
}

// BuildingLockPath returns the path of the lock file written during a build.
func BuildingLockPath(projectDir, arch, name string) string {
	return filepath.Join(UnitBuildDir(projectDir, arch, name), ".lock")
}

// IsBuildInProgress returns true if another process is currently building this unit.
// It checks for the lock file and verifies the PID is still alive.
func IsBuildInProgress(projectDir, arch, name string) bool {
	data, err := os.ReadFile(BuildingLockPath(projectDir, arch, name))
	if err != nil {
		return false
	}
	pid := strings.TrimSpace(string(data))
	// Check if the process is still running
	_, err = os.Stat(fmt.Sprintf("/proc/%s", pid))
	return err == nil
}

func writeCacheMarker(projectDir, arch, name, hash string) {
	path := CacheMarkerPath(projectDir, arch, name, hash)
	EnsureDir(filepath.Dir(path))
	os.WriteFile(path, []byte(hash), 0644)
}


// repoRelPath returns the repo directory path relative to the project root.
func repoRelPath(proj *yoestar.Project, projectDir string) string {
	repoDir := repo.RepoDir(proj, projectDir)
	rel, err := filepath.Rel(projectDir, repoDir)
	if err != nil {
		return "repo"
	}
	return rel
}
