package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/YoeDistro/yoe-ng/internal/image"
	"github.com/YoeDistro/yoe-ng/internal/packaging"
	"github.com/YoeDistro/yoe-ng/internal/repo"
	"github.com/YoeDistro/yoe-ng/internal/resolve"
	"github.com/YoeDistro/yoe-ng/internal/source"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// Options controls build behavior.
type Options struct {
	Force      bool   // rebuild even if cached
	Clean      bool   // delete build dir before rebuilding (implies Force)
	NoCache    bool   // skip all caches
	DryRun     bool   // show what would be built
	Verbose    bool   // show build output in console (default: log only)
	ProjectDir string // project root
	Arch       string // target architecture
}

// BuildRecipes builds the specified recipes (or all if names is empty).
func BuildRecipes(proj *yoestar.Project, names []string, opts Options, w io.Writer) error {
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

	// Filter to requested recipes (and their deps)
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

	// Build in order
	for _, name := range order {
		recipe := proj.Recipes[name]
		hash := hashes[name]

		// --force/--clean only apply to explicitly requested recipes;
		// dependencies still use the cache.
		forceThis := (opts.Force || opts.Clean) && (len(requested) == 0 || requested[name])

		if !forceThis && !opts.NoCache {
			if isBuildCached(opts.ProjectDir, name, hash) {
				fmt.Fprintf(w, "%-20s [cached] %s\n", name, hash[:12])
				continue
			}
		}

		fmt.Fprintf(w, "%-20s [building]\n", name)

		if err := buildOne(proj, recipe, hash, opts, w); err != nil {
			return fmt.Errorf("building %s: %w", name, err)
		}

		// Write cache marker
		writeCacheMarker(opts.ProjectDir, name, hash)
		fmt.Fprintf(w, "%-20s [done] %s\n", name, hash[:12])
	}

	return nil
}

func buildOne(proj *yoestar.Project, recipe *yoestar.Recipe, hash string, opts Options, w io.Writer) error {
	buildDir := RecipeBuildDir(opts.ProjectDir, recipe.Name)
	EnsureDir(buildDir)

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

	// Image recipes go through a different path — assemble rootfs
	if recipe.Class == "image" {
		outputDir := filepath.Join(buildDir, "output")
		if err := image.Assemble(recipe, proj, opts.ProjectDir, outputDir, logW); err != nil {
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
	// Recipes without a source field (e.g., musl) skip this step.
	if recipe.Source != "" {
		if _, err := source.Prepare(opts.ProjectDir, recipe); err != nil {
			return fmt.Errorf("preparing source: %w", err)
		}
	} else {
		EnsureDir(srcDir)
	}

	// Determine build commands based on class
	commands := buildCommands(recipe)
	if len(commands) == 0 {
		fmt.Fprintf(w, "  (no build steps for %s class %q)\n", recipe.Name, recipe.Class)
		return nil
	}

	// Build environment.
	// The sysroot at /build/sysroot contains headers/libs from built deps.
	sysroot := SysrootDir(opts.ProjectDir)
	EnsureDir(sysroot)
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
	}

	// Execute each build step inside the container with bwrap
	for i, cmd := range commands {
		fmt.Fprintf(logW, "  [%d/%d] %s\n", i+1, len(commands), cmd)

		cfg := &SandboxConfig{
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
	if recipe.Class != "image" {
		apkPath, err := packaging.CreateAPK(recipe, destDir, filepath.Join(buildDir, "pkg"))
		if err != nil {
			return fmt.Errorf("creating apk: %w", err)
		}
		fmt.Fprintf(w, "  → %s\n", filepath.Base(apkPath))

		repoDir := repo.RepoDir(nil, opts.ProjectDir)
		if err := repo.Publish(apkPath, repoDir); err != nil {
			return fmt.Errorf("publishing to repo: %w", err)
		}

		// Install into the shared sysroot so subsequent builds can find
		// this package's headers, libraries, and pkg-config files.
		if err := InstallToSysroot(destDir, sysroot); err != nil {
			fmt.Fprintf(w, "  (warning: sysroot install failed: %v)\n", err)
		}
	}

	return nil
}

// buildCommands returns the shell commands to execute for a recipe.
func buildCommands(recipe *yoestar.Recipe) []string {
	// Explicit build steps take priority
	if len(recipe.Build) > 0 {
		return recipe.Build
	}

	// Class-specific defaults
	switch recipe.Class {
	case "autotools":
		configureArgs := ""
		if len(recipe.ConfigureArgs) > 0 {
			configureArgs = " " + joinArgs(recipe.ConfigureArgs)
		}
		return []string{
			"./configure --prefix=$PREFIX" + configureArgs,
			"make -j$NPROC",
			"make DESTDIR=$DESTDIR install",
		}
	case "cmake":
		cmakeArgs := ""
		if len(recipe.ConfigureArgs) > 0 {
			cmakeArgs = " " + joinArgs(recipe.ConfigureArgs)
		}
		return []string{
			"cmake -B build -DCMAKE_INSTALL_PREFIX=$PREFIX" + cmakeArgs,
			"cmake --build build -j $NPROC",
			"DESTDIR=$DESTDIR cmake --install build",
		}
	case "go":
		pkg := recipe.GoPackage
		if pkg == "" {
			pkg = "."
		}
		return []string{
			fmt.Sprintf("go build -o $DESTDIR/usr/bin/%s %s", recipe.Name, pkg),
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
			return nil, fmt.Errorf("recipe %q not found", name)
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

func dryRun(w io.Writer, proj *yoestar.Project, order []string, hashes map[string]string, opts Options, requested map[string]bool) error {
	fmt.Fprintln(w, "Dry run — would build in this order:")
	for _, name := range order {
		recipe := proj.Recipes[name]
		cached := ""
		forceThis := (opts.Force || opts.Clean) && (len(requested) == 0 || requested[name])
		if !forceThis && isBuildCached(opts.ProjectDir, name, hashes[name]) {
			cached = " [cached, skip]"
		}
		fmt.Fprintf(w, "  %-20s [%s] %s%s\n", name, recipe.Class, hashes[name][:12], cached)
	}
	return nil
}

// --- Simple file-based cache ---

func cacheMarkerPath(projectDir, name, hash string) string {
	return filepath.Join(projectDir, "build", name, ".yoe-hash")
}

func isBuildCached(projectDir, name, hash string) bool {
	data, err := os.ReadFile(cacheMarkerPath(projectDir, name, hash))
	if err != nil {
		return false
	}
	return string(data) == hash
}

func writeCacheMarker(projectDir, name, hash string) {
	path := cacheMarkerPath(projectDir, name, hash)
	EnsureDir(filepath.Dir(path))
	os.WriteFile(path, []byte(hash), 0644)
}
