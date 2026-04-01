package starlark

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
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

	// Set MACHINE variable so image definitions can conditionally include
	// board-specific units (e.g., different kernels per RPi board).
	machine := ""
	if proj := eng.Project(); proj != nil {
		machine = proj.Defaults.Machine
	}
	eng.SetVar("MACHINE", starlark.String(machine))

	// Set MACHINE_CONFIG — a Starlark struct exposing the active machine's
	// configuration to unit and image definitions.
	if proj := eng.Project(); proj != nil {
		if m, ok := eng.Machines()[proj.Defaults.Machine]; ok {
			machineDict := starlark.StringDict{
				"name":     starlark.String(m.Name),
				"arch":     starlark.String(m.Arch),
				"packages": toStarlarkStringList(m.Packages),
			}
			// Add partitions as a Starlark list
			var partList []starlark.Value
			for _, p := range m.Partitions {
				fields := starlark.StringDict{
					"label": starlark.String(p.Label),
					"type":  starlark.String(p.Type),
					"size":  starlark.String(p.Size),
					"root":  starlark.Bool(p.Root),
				}
				if len(p.Contents) > 0 {
					fields["contents"] = toStarlarkStringList(p.Contents)
				}
				partList = append(partList, starlarkstruct.FromStringDict(starlark.String("partition"), fields))
			}
			machineDict["partitions"] = starlark.NewList(partList)

			// Add kernel info
			if m.Kernel.Unit != "" {
				machineDict["kernel"] = starlarkstruct.FromStringDict(
					starlark.String("kernel"), starlark.StringDict{
						"unit":      starlark.String(m.Kernel.Unit),
						"provides":  starlark.String(m.Kernel.Provides),
						"defconfig": starlark.String(m.Kernel.Defconfig),
						"cmdline":   starlark.String(m.Kernel.Cmdline),
					})
			}

			eng.SetVar("MACHINE_CONFIG", starlarkstruct.FromStringDict(
				starlark.String("machine_config"), machineDict))
		}
	}

	// Set PROVIDES — a Starlark dict mapping virtual package names to concrete
	// unit names. Initially populated from kernel.provides; updated after phase 2
	// with unit provides.
	provides := starlark.NewDict(4)
	if proj := eng.Project(); proj != nil {
		if m, ok := eng.Machines()[proj.Defaults.Machine]; ok {
			if m.Kernel.Provides != "" {
				_ = provides.SetKey(starlark.String(m.Kernel.Provides),
					starlark.String(m.Kernel.Unit))
			}
		}
	}
	eng.SetVar("PROVIDES", provides)

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

	// After evaluating units/images, add unit provides to PROVIDES dict
	if prov, ok := eng.vars["PROVIDES"].(*starlark.Dict); ok {
		for _, u := range eng.Units() {
			if u.Provides != "" {
				_ = prov.SetKey(starlark.String(u.Provides), starlark.String(u.Name))
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

func toStarlarkStringList(ss []string) *starlark.List {
	vals := make([]starlark.Value, len(ss))
	for i, s := range ss {
		vals[i] = starlark.String(s)
	}
	return starlark.NewList(vals)
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
