package bootstrap

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/YoeDistro/yoe-ng/internal/build"
	"github.com/YoeDistro/yoe-ng/internal/packaging"
	"github.com/YoeDistro/yoe-ng/internal/repo"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// Bootstrap recipe names — the minimal set needed for a self-hosting build root.
var bootstrapRecipes = []string{
	"linux-headers",
	"glibc",
	"binutils",
	"gcc",
	"busybox",
	"apk-tools",
	"bubblewrap",
}

// Stage0 builds the initial base packages using the host's toolchain (Alpine's
// gcc inside the container). The output is a minimal set of .apk files — enough
// to create a self-hosting Yoe-NG build root.
func Stage0(proj *yoestar.Project, projectDir string, w io.Writer) error {
	fmt.Fprintln(w, "=== Bootstrap Stage 0: Cross-Pollination ===")
	fmt.Fprintln(w, "Building base packages using host toolchain...")
	fmt.Fprintln(w)

	arch := build.Arch()

	// Verify bootstrap recipes exist
	var missing []string
	for _, name := range bootstrapRecipes {
		if _, ok := proj.Recipes[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("bootstrap recipes not found: %s\nAdd them to your project or include a layer that provides them",
			strings.Join(missing, ", "))
	}

	// Build each bootstrap recipe without sandbox isolation (using host tools)
	repoDir := repo.RepoDir(proj, projectDir)

	for _, name := range bootstrapRecipes {
		recipe := proj.Recipes[name]
		fmt.Fprintf(w, "\n--- Building %s %s ---\n", recipe.Name, recipe.Version)

		buildDir := build.RecipeBuildDir(projectDir, recipe.Name)
		destDir := filepath.Join(buildDir, "destdir")

		// Clean and prepare
		os.RemoveAll(destDir)
		os.MkdirAll(destDir, 0755)

		// Get build commands
		commands := stage0Commands(recipe)
		if len(commands) == 0 {
			fmt.Fprintf(w, "  (no build steps for %s)\n", recipe.Name)
			continue
		}

		// Build environment — use host tools directly, no sandbox
		env := map[string]string{
			"PREFIX":  "/usr",
			"DESTDIR": destDir,
			"NPROC":   build.NProc(),
			"ARCH":    arch,
			"HOME":    "/tmp",
		}

		for i, cmd := range commands {
			fmt.Fprintf(w, "  [%d/%d] %s\n", i+1, len(commands), cmd)
			if err := build.RunSimple(buildDir, destDir, env, cmd); err != nil {
				return fmt.Errorf("stage0 %s step %d: %w", recipe.Name, i+1, err)
			}
		}

		// Package the output
		apkPath, err := packaging.CreateAPK(recipe, destDir, filepath.Join(buildDir, "pkg"))
		if err != nil {
			return fmt.Errorf("packaging %s: %w", recipe.Name, err)
		}

		// Publish to the local repo
		if err := repo.Publish(apkPath, repoDir); err != nil {
			return fmt.Errorf("publishing %s: %w", recipe.Name, err)
		}

		fmt.Fprintf(w, "  → %s\n", filepath.Base(apkPath))
	}

	fmt.Fprintf(w, "\n=== Stage 0 complete: %d packages in %s ===\n", len(bootstrapRecipes), repoDir)
	return nil
}

// Stage1 rebuilds the base packages using the Stage 0 packages. After this,
// all packages in the repository were built by Yoe-NG's own toolchain.
func Stage1(proj *yoestar.Project, projectDir string, w io.Writer) error {
	fmt.Fprintln(w, "=== Bootstrap Stage 1: Self-Hosting Rebuild ===")
	fmt.Fprintln(w, "Rebuilding base packages using Yoe-NG's own toolchain...")
	fmt.Fprintln(w)

	arch := build.Arch()
	repoDir := repo.RepoDir(proj, projectDir)

	// Verify Stage 0 packages exist in the repo
	if err := verifyStage0(repoDir); err != nil {
		return fmt.Errorf("stage 0 not complete: %w\nRun 'yoe bootstrap stage0' first", err)
	}

	// Create a Yoe-NG build root from Stage 0 packages
	buildRoot := filepath.Join(projectDir, "build", "bootstrap", "buildroot")
	if err := createBuildRoot(buildRoot, repoDir, w); err != nil {
		return fmt.Errorf("creating build root: %w", err)
	}

	// Rebuild each bootstrap recipe inside the build root
	for _, name := range bootstrapRecipes {
		recipe := proj.Recipes[name]
		fmt.Fprintf(w, "\n--- Rebuilding %s %s (self-hosted) ---\n", recipe.Name, recipe.Version)

		buildDir := build.RecipeBuildDir(projectDir, recipe.Name)
		destDir := filepath.Join(buildDir, "destdir")

		os.RemoveAll(destDir)
		os.MkdirAll(destDir, 0755)

		commands := stage0Commands(recipe)
		if len(commands) == 0 {
			continue
		}

		// Build inside the Yoe-NG build root using bubblewrap
		env := map[string]string{
			"PREFIX":  "/usr",
			"DESTDIR": "/build/destdir",
			"NPROC":   build.NProc(),
			"ARCH":    arch,
			"HOME":    "/tmp",
		}

		for i, cmd := range commands {
			fmt.Fprintf(w, "  [%d/%d] %s\n", i+1, len(commands), cmd)

			if build.HasBwrap() {
				cfg := &build.SandboxConfig{
					BuildRoot: buildRoot,
					SrcDir:    buildDir,
					DestDir:   destDir,
					Env:       env,
				}
				if err := build.RunInSandbox(cfg, cmd); err != nil {
					return fmt.Errorf("stage1 %s step %d: %w", recipe.Name, i+1, err)
				}
			} else {
				env["DESTDIR"] = destDir
				if err := build.RunSimple(buildDir, destDir, env, cmd); err != nil {
					return fmt.Errorf("stage1 %s step %d: %w", recipe.Name, i+1, err)
				}
			}
		}

		// Package and publish (overwriting Stage 0 packages)
		apkPath, err := packaging.CreateAPK(recipe, destDir, filepath.Join(buildDir, "pkg"))
		if err != nil {
			return fmt.Errorf("packaging %s: %w", recipe.Name, err)
		}
		if err := repo.Publish(apkPath, repoDir); err != nil {
			return fmt.Errorf("publishing %s: %w", recipe.Name, err)
		}

		fmt.Fprintf(w, "  → %s (self-hosted)\n", filepath.Base(apkPath))
	}

	fmt.Fprintf(w, "\n=== Stage 1 complete: all base packages rebuilt with Yoe-NG toolchain ===\n")
	return nil
}

// Status shows the current bootstrap state.
func Status(proj *yoestar.Project, projectDir string, w io.Writer) error {
	repoDir := repo.RepoDir(proj, projectDir)

	fmt.Fprintf(w, "Bootstrap status for %s\n\n", proj.Name)
	fmt.Fprintf(w, "Repository: %s\n", repoDir)
	fmt.Fprintf(w, "Architecture: %s\n\n", build.Arch())

	for _, name := range bootstrapRecipes {
		status := "missing"
		if _, ok := proj.Recipes[name]; ok {
			status = "recipe found"
		}

		// Check if package exists in repo
		entries, err := os.ReadDir(repoDir)
		if err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), name+"-") && strings.HasSuffix(e.Name(), ".apk") {
					status = "built (" + e.Name() + ")"
					break
				}
			}
		}

		fmt.Fprintf(w, "  %-16s %s\n", name, status)
	}

	return nil
}

// stage0Commands returns the build commands for a bootstrap recipe.
// Bootstrap builds use the host toolchain directly, so we use the recipe's
// build steps if available, or class-specific defaults.
func stage0Commands(recipe *yoestar.Recipe) []string {
	if len(recipe.Build) > 0 {
		return recipe.Build
	}

	switch recipe.Class {
	case "autotools":
		args := ""
		if len(recipe.ConfigureArgs) > 0 {
			args = " " + strings.Join(recipe.ConfigureArgs, " ")
		}
		return []string{
			"./configure --prefix=$PREFIX" + args,
			"make -j$NPROC",
			"make DESTDIR=$DESTDIR install",
		}
	case "cmake":
		args := ""
		if len(recipe.ConfigureArgs) > 0 {
			args = " " + strings.Join(recipe.ConfigureArgs, " ")
		}
		return []string{
			"cmake -B build -DCMAKE_INSTALL_PREFIX=$PREFIX" + args,
			"cmake --build build -j $NPROC",
			"DESTDIR=$DESTDIR cmake --install build",
		}
	}

	return nil
}

func verifyStage0(repoDir string) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return fmt.Errorf("repo not found at %s", repoDir)
	}

	for _, name := range bootstrapRecipes {
		found := false
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), name+"-") && strings.HasSuffix(e.Name(), ".apk") {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("package %s not found in repo", name)
		}
	}

	return nil
}

func createBuildRoot(buildRoot, repoDir string, w io.Writer) error {
	fmt.Fprintf(w, "Creating build root at %s...\n", buildRoot)

	os.RemoveAll(buildRoot)
	os.MkdirAll(buildRoot, 0755)

	// Check if apk is available
	if _, err := exec.LookPath("apk"); err != nil {
		// Fallback: just extract packages into the build root
		fmt.Fprintln(w, "  (apk not available — extracting packages manually)")
		return extractPackages(buildRoot, repoDir)
	}

	// Use apk to install base packages into the build root
	args := []string{
		"--root", buildRoot,
		"--initdb",
		"--no-scripts",
		"--no-cache",
		"--repository", repoDir,
		"add",
	}
	args = append(args, bootstrapRecipes...)

	cmd := exec.Command("apk", args...)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func extractPackages(buildRoot, repoDir string) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".apk") {
			continue
		}
		apkPath := filepath.Join(repoDir, e.Name())
		cmd := exec.Command("tar", "xzf", apkPath, "-C", buildRoot)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("extracting %s: %s\n%s", e.Name(), err, out)
		}
	}

	return nil
}
