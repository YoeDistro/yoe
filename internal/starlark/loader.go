package starlark

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
)

// LoadOption configures optional behavior for LoadProject / LoadProjectFromRoot.
type LoadOption func(*loadConfig)

type loadConfig struct {
	layerSync func([]LayerRef, io.Writer) error
	machine   string // override default machine before evaluating units/images
}

// WithLayerSync provides a callback that is invoked after PROJECT.star is
// evaluated to ensure all declared layers are available (e.g. cloned).
// The callback receives the layer list and a writer for progress output.
func WithLayerSync(fn func([]LayerRef, io.Writer) error) LoadOption {
	return func(c *loadConfig) { c.layerSync = fn }
}

// WithMachine overrides the project's default machine before units and
// images are evaluated. This allows target_arch() in Starlark to return
// the correct architecture for the specified machine.
func WithMachine(name string) LoadOption {
	return func(c *loadConfig) { c.machine = name }
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

	// Phase 1: Evaluate all machine definitions (project + layers).
	// Machines must be loaded before units/images so that target_arch()
	// returns the correct value during Starlark evaluation.
	if err := evalDir(eng, root, "machines"); err != nil {
		return nil, err
	}
	if eng.layerRoots != nil {
		for _, layerPath := range eng.layerRoots {
			if err := evalDir(eng, layerPath, "machines"); err != nil {
				return nil, err
			}
		}
	}

	// Apply machine override before evaluating units/images.
	if cfg.machine != "" {
		if proj := eng.Project(); proj != nil {
			proj.Defaults.Machine = cfg.machine
		}
	}

	// Set ARCH variable for phase 2 so Starlark files can use it
	// (e.g., conditional artifacts in image definitions).
	// Always set a value — default to x86_64 if no machine is configured.
	arch := "x86_64"
	if proj := eng.Project(); proj != nil {
		if m, ok := eng.Machines()[proj.Defaults.Machine]; ok {
			arch = m.Arch
		}
	}
	eng.SetVar("ARCH", starlark.String(arch))

	// Phase 2: Evaluate units and images (project + layers).
	if err := evalDir(eng, root, "units"); err != nil {
		return nil, err
	}
	if eng.layerRoots != nil {
		for _, layerPath := range eng.layerRoots {
			for _, subdir := range []string{"units", "images"} {
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
