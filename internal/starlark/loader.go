package starlark

import (
	"fmt"
	"os"
	"path/filepath"
)

// LoadProject finds the project root, evaluates all .star files, and returns
// a fully populated Project.
func LoadProject(startDir string) (*Project, error) {
	root, err := findProjectRoot(startDir)
	if err != nil {
		return nil, err
	}

	return LoadProjectFromRoot(root)
}

// findProjectRoot walks up from startDir looking for PROJECT.star.
func findProjectRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	for {
		candidate := filepath.Join(dir, "PROJECT.star")
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no PROJECT.star found in %s or any parent directory", startDir)
}

// LoadProjectFromRoot evaluates all .star files under a known project root
// and returns a fully populated Project. Unlike LoadProject, it does not
// search for PROJECT.star — the caller must provide the exact root directory.
func LoadProjectFromRoot(root string) (*Project, error) {
	eng := NewEngine()
	eng.SetProjectRoot(root)

	// Evaluate PROJECT.star first
	projFile := filepath.Join(root, "PROJECT.star")
	if err := eng.ExecFile(projFile); err != nil {
		return nil, fmt.Errorf("evaluating PROJECT.star: %w", err)
	}

	// Register local layer roots so load("@layer//...") works
	if proj := eng.Project(); proj != nil {
		for _, l := range proj.Layers {
			if l.Local != "" {
				layerPath := l.Local
				if !filepath.IsAbs(layerPath) {
					layerPath = filepath.Join(root, layerPath)
				}
				name := filepath.Base(l.URL)
				eng.SetLayerRoot(name, layerPath)
			}
		}
	}

	// Evaluate all machine definitions
	if err := evalDir(eng, root, "machines"); err != nil {
		return nil, err
	}

	// Evaluate all recipes
	if err := evalDir(eng, root, "recipes"); err != nil {
		return nil, err
	}

	// Evaluate machines, recipes, and images from local layers
	if eng.layerRoots != nil {
		for _, layerPath := range eng.layerRoots {
			for _, subdir := range []string{"machines", "recipes", "images"} {
				if err := evalDir(eng, layerPath, subdir); err != nil {
					return nil, err
				}
			}
		}
	}

	proj := eng.Project()
	if proj == nil {
		return nil, fmt.Errorf("PROJECT.star did not call project()")
	}

	proj.Machines = eng.Machines()
	proj.Recipes = eng.Recipes()

	return proj, nil
}

func evalDir(eng *Engine, root, subdir string) error {
	pattern := filepath.Join(root, subdir, "*.star")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("globbing %s: %w", pattern, err)
	}
	for _, path := range matches {
		if err := eng.ExecFile(path); err != nil {
			return fmt.Errorf("evaluating %s: %w", path, err)
		}
	}
	return nil
}
