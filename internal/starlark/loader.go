package starlark

import (
	"fmt"
	"path/filepath"

	"github.com/YoeDistro/yoe-ng/internal/config"
)

// LoadProject finds the project root, evaluates all .star files, and returns
// a fully populated Project.
func LoadProject(startDir string) (*Project, error) {
	root, err := config.FindProjectRoot(startDir)
	if err != nil {
		return nil, err
	}

	eng := NewEngine()

	// Evaluate PROJECT.star first
	projFile := filepath.Join(root, "PROJECT.star")
	if err := eng.ExecFile(projFile); err != nil {
		return nil, fmt.Errorf("evaluating PROJECT.star: %w", err)
	}

	// Evaluate all machine definitions
	if err := evalDir(eng, root, "machines"); err != nil {
		return nil, err
	}

	// Evaluate all recipes
	if err := evalDir(eng, root, "recipes"); err != nil {
		return nil, err
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
