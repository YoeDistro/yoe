package starlark

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LoadOption configures optional behavior for LoadProject / LoadProjectFromRoot.
type LoadOption func(*loadConfig)

type loadConfig struct {
	layerSync func([]LayerRef, io.Writer) error
}

// WithLayerSync provides a callback that is invoked after PROJECT.star is
// evaluated to ensure all declared layers are available (e.g. cloned).
// The callback receives the layer list and a writer for progress output.
func WithLayerSync(fn func([]LayerRef, io.Writer) error) LoadOption {
	return func(c *loadConfig) { c.layerSync = fn }
}

// LoadProject finds the project root, evaluates all .star files, and returns
// a fully populated Project.
func LoadProject(startDir string, opts ...LoadOption) (*Project, error) {
	root, err := findProjectRoot(startDir)
	if err != nil {
		return nil, err
	}

	return LoadProjectFromRoot(root, opts...)
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
func LoadProjectFromRoot(root string, opts ...LoadOption) (*Project, error) {
	var cfg loadConfig
	for _, o := range opts {
		o(&cfg)
	}

	eng := NewEngine()
	eng.SetProjectRoot(root)

	// Evaluate PROJECT.star first
	projFile := filepath.Join(root, "PROJECT.star")
	if err := eng.ExecFile(projFile); err != nil {
		return nil, fmt.Errorf("evaluating PROJECT.star: %w", err)
	}

	// Sync layers if a sync callback was provided (auto-clone missing layers).
	if cfg.layerSync != nil {
		if proj := eng.Project(); proj != nil && len(proj.Layers) > 0 {
			if err := cfg.layerSync(proj.Layers, os.Stderr); err != nil {
				return nil, fmt.Errorf("syncing layers: %w", err)
			}
		}
	}

	// Register layer roots so load("@layer//...") works.
	// Check local overrides first, then the layer cache.
	if proj := eng.Project(); proj != nil {
		for _, l := range proj.Layers {
			// Derive layer name: use Path's last component if set, otherwise URL's
			name := filepath.Base(strings.TrimSuffix(l.URL, ".git"))
			if l.Path != "" {
				name = filepath.Base(l.Path)
			}

			if l.Local != "" {
				layerPath := l.Local
				if !filepath.IsAbs(layerPath) {
					layerPath = filepath.Join(root, layerPath)
				}
				if l.Path != "" {
					layerPath = filepath.Join(layerPath, l.Path)
				}
				eng.SetLayerRoot(name, layerPath)
				continue
			}

			// Check layer cache
			cacheDir := os.Getenv("YOE_CACHE")
			if cacheDir == "" {
				cacheDir = "cache"
			}
			layerDir := filepath.Join(cacheDir, "layers", name)
			if l.Path != "" {
				layerDir = filepath.Join(layerDir, l.Path)
			}
			if _, err := os.Stat(layerDir); err == nil {
				eng.SetLayerRoot(name, layerDir)
			}
		}
	}

	// Evaluate all machine definitions
	if err := evalDir(eng, root, "machines"); err != nil {
		return nil, err
	}

	// Evaluate all units
	if err := evalDir(eng, root, "units"); err != nil {
		return nil, err
	}

	// Evaluate machines, units, and images from local layers
	if eng.layerRoots != nil {
		for _, layerPath := range eng.layerRoots {
			for _, subdir := range []string{"machines", "units", "images"} {
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
	proj.Units = eng.Units()

	return proj, nil
}

func evalDir(eng *Engine, root, subdir string) error {
	base := filepath.Join(root, subdir)
	return filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".star") {
			return nil
		}
		if err := eng.ExecFile(path); err != nil {
			return fmt.Errorf("evaluating %s: %w", path, err)
		}
		return nil
	})
}
