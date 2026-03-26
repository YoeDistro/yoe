package internal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// DevExtract extracts local commits in a recipe's build directory as patch
// files and updates the recipe's patches list.
func DevExtract(projectDir, recipeName string, w io.Writer) error {
	proj, err := yoestar.LoadProject(projectDir)
	if err != nil {
		return err
	}

	recipe, ok := proj.Recipes[recipeName]
	if !ok {
		return fmt.Errorf("recipe %q not found", recipeName)
	}

	srcDir := recipeSrcDir(projectDir, recipeName)
	if _, err := os.Stat(filepath.Join(srcDir, ".git")); os.IsNotExist(err) {
		return fmt.Errorf("%s is not a git repo — build the recipe first with yoe build", srcDir)
	}

	// Check if there are commits beyond upstream
	out, err := gitCmd(srcDir, "rev-list", "upstream..HEAD")
	if err != nil {
		return fmt.Errorf("no 'upstream' tag in %s — was this source fetched by yoe?", srcDir)
	}
	commits := strings.TrimSpace(out)
	if commits == "" {
		fmt.Fprintf(w, "No local commits beyond upstream in %s\n", recipeName)
		return nil
	}

	// Create patches directory
	patchDir := filepath.Join(projectDir, "patches", recipeName)
	if err := os.MkdirAll(patchDir, 0755); err != nil {
		return fmt.Errorf("creating patch directory: %w", err)
	}

	// Remove old patches
	oldPatches, _ := filepath.Glob(filepath.Join(patchDir, "*.patch"))
	for _, p := range oldPatches {
		os.Remove(p)
	}

	// Extract patches with git format-patch
	_, err = gitCmd(srcDir, "format-patch", "--output-directory", patchDir, "upstream..HEAD")
	if err != nil {
		return fmt.Errorf("git format-patch: %w", err)
	}

	// List generated patches
	patches, _ := filepath.Glob(filepath.Join(patchDir, "*.patch"))
	if len(patches) == 0 {
		fmt.Fprintf(w, "No patches extracted\n")
		return nil
	}

	// Build the patches list relative to project root
	var patchPaths []string
	for _, p := range patches {
		rel, _ := filepath.Rel(projectDir, p)
		patchPaths = append(patchPaths, rel)
		fmt.Fprintf(w, "  %s\n", rel)
	}

	fmt.Fprintf(w, "\nExtracted %d patch(es) for %s\n", len(patches), recipeName)
	fmt.Fprintf(w, "Update your recipe's patches list to:\n")
	fmt.Fprintf(w, "    patches = [\n")
	for _, p := range patchPaths {
		fmt.Fprintf(w, "        %q,\n", p)
	}
	fmt.Fprintf(w, "    ],\n")

	// Check if recipe already had patches and show diff
	if len(recipe.Patches) > 0 {
		fmt.Fprintf(w, "\nPrevious patches were:\n")
		for _, p := range recipe.Patches {
			fmt.Fprintf(w, "    %q,\n", p)
		}
	}

	return nil
}

// DevDiff shows local commits beyond upstream in a recipe's build directory.
func DevDiff(projectDir, recipeName string, w io.Writer) error {
	srcDir := recipeSrcDir(projectDir, recipeName)
	if _, err := os.Stat(filepath.Join(srcDir, ".git")); os.IsNotExist(err) {
		return fmt.Errorf("%s is not a git repo — build the recipe first", srcDir)
	}

	out, err := gitCmd(srcDir, "log", "--oneline", "upstream..HEAD")
	if err != nil {
		return fmt.Errorf("no 'upstream' tag in %s", srcDir)
	}

	if strings.TrimSpace(out) == "" {
		fmt.Fprintf(w, "No local changes beyond upstream in %s\n", recipeName)
		return nil
	}

	fmt.Fprintf(w, "Local commits in %s (upstream..HEAD):\n\n", recipeName)
	fmt.Fprint(w, out)
	return nil
}

// DevStatus shows which recipes have local modifications.
func DevStatus(projectDir string, w io.Writer) error {
	proj, err := yoestar.LoadProject(projectDir)
	if err != nil {
		return err
	}

	buildDir := filepath.Join(projectDir, "build")
	found := false

	for name := range proj.Recipes {
		srcDir := filepath.Join(buildDir, name, "src")
		gitDir := filepath.Join(srcDir, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			continue
		}

		out, err := gitCmd(srcDir, "rev-list", "--count", "upstream..HEAD")
		if err != nil {
			continue
		}

		count := strings.TrimSpace(out)
		if count != "0" {
			fmt.Fprintf(w, "%-20s %s commit(s) ahead of upstream\n", name, count)
			found = true
		}
	}

	if !found {
		fmt.Fprintln(w, "No recipes with local modifications")
	}

	return nil
}

func recipeSrcDir(projectDir, recipeName string) string {
	return filepath.Join(projectDir, "build", recipeName, "src")
}

func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
