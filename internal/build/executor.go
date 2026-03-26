package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/YoeDistro/yoe-ng/internal/resolve"
	"github.com/YoeDistro/yoe-ng/internal/source"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// Options controls build behavior.
type Options struct {
	Force         bool   // rebuild even if cached
	NoCache       bool   // skip all caches
	DryRun        bool   // show what would be built
	UseSandbox    bool   // use bubblewrap sandbox
	ProjectDir    string // project root
	Arch          string // target architecture
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
	if len(names) > 0 {
		order, err = filterBuildOrder(dag, order, names)
		if err != nil {
			return err
		}
	}

	if opts.DryRun {
		return dryRun(w, proj, order, hashes, opts)
	}

	// Build in order
	for _, name := range order {
		recipe := proj.Recipes[name]
		hash := hashes[name]

		// Check cache
		if !opts.Force && !opts.NoCache {
			if isBuildCached(opts.ProjectDir, name, hash) {
				fmt.Fprintf(w, "%-20s [cached] %s\n", name, hash[:12])
				continue
			}
		}

		fmt.Fprintf(w, "%-20s [building]\n", name)

		if err := buildOne(recipe, hash, opts, w); err != nil {
			return fmt.Errorf("building %s: %w", name, err)
		}

		// Write cache marker
		writeCacheMarker(opts.ProjectDir, name, hash)
		fmt.Fprintf(w, "%-20s [done] %s\n", name, hash[:12])
	}

	return nil
}

func buildOne(recipe *yoestar.Recipe, hash string, opts Options, w io.Writer) error {
	buildDir := RecipeBuildDir(opts.ProjectDir, recipe.Name)
	srcDir := filepath.Join(buildDir, "src")
	destDir := filepath.Join(buildDir, "destdir")

	// Clean destdir
	os.RemoveAll(destDir)
	EnsureDir(destDir)

	// Prepare source (fetch + extract + patch, or reuse dev source)
	if _, err := source.Prepare(opts.ProjectDir, recipe); err != nil {
		return fmt.Errorf("preparing source: %w", err)
	}

	// Determine build commands based on class
	commands := buildCommands(recipe)
	if len(commands) == 0 {
		fmt.Fprintf(w, "  (no build steps for %s class %q)\n", recipe.Name, recipe.Class)
		return nil
	}

	// Build environment
	env := map[string]string{
		"PREFIX":  "/usr",
		"DESTDIR": "/build/destdir",
		"NPROC":   NProc(),
		"ARCH":    opts.Arch,
		"HOME":    "/tmp",
	}

	// Execute each build step
	for i, cmd := range commands {
		fmt.Fprintf(w, "  [%d/%d] %s\n", i+1, len(commands), cmd)

		if opts.UseSandbox && HasBwrap() {
			cfg := &SandboxConfig{
				SrcDir:  srcDir,
				DestDir: destDir,
				Env:     env,
			}
			if err := RunInSandbox(cfg, cmd); err != nil {
				return err
			}
		} else {
			// Set DESTDIR to actual path when not sandboxed
			env["DESTDIR"] = destDir
			if err := RunSimple(srcDir, destDir, env, cmd); err != nil {
				return err
			}
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

func dryRun(w io.Writer, proj *yoestar.Project, order []string, hashes map[string]string, opts Options) error {
	fmt.Fprintln(w, "Dry run — would build in this order:")
	for _, name := range order {
		recipe := proj.Recipes[name]
		cached := ""
		if !opts.Force && isBuildCached(opts.ProjectDir, name, hashes[name]) {
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
