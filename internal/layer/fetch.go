package layer

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// CacheDir returns the layer cache directory.
func CacheDir() (string, error) {
	dir := os.Getenv("YOE_CACHE")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".cache", "yoe-ng")
	}
	dir = filepath.Join(dir, "layers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// Sync fetches all layers declared in the project. For each layer:
// - If Local is set, skip (use the local path directly)
// - Otherwise, git clone/fetch into $YOE_CACHE/layers/<name>/
// Returns a map of layer name → directory path.
func Sync(proj *yoestar.Project, w io.Writer) (map[string]string, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)

	for _, l := range proj.Layers {
		name := LayerName(l)

		if l.Local != "" {
			fmt.Fprintf(w, "  %-20s (local: %s)\n", name, l.Local)
			result[name] = l.Local
			continue
		}

		layerDir := filepath.Join(cacheDir, name)
		ref := l.Ref
		if ref == "" {
			ref = "main"
		}

		if _, err := os.Stat(filepath.Join(layerDir, ".git")); os.IsNotExist(err) {
			// Clone
			fmt.Fprintf(w, "  %-20s cloning %s (ref: %s)...\n", name, l.URL, ref)
			cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, l.URL, layerDir)
			cmd.Stdout = w
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return nil, fmt.Errorf("cloning layer %s: %w", name, err)
			}
		} else {
			// Fetch and checkout the right ref
			fmt.Fprintf(w, "  %-20s fetching %s...\n", name, ref)
			cmd := exec.Command("git", "fetch", "origin", ref)
			cmd.Dir = layerDir
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return nil, fmt.Errorf("fetching layer %s: %w", name, err)
			}

			cmd = exec.Command("git", "checkout", "FETCH_HEAD")
			cmd.Dir = layerDir
			cmd.Stderr = os.Stderr
			cmd.Run() // best effort
		}

		// If layer specifies a subdirectory path, use that
		layerRoot := layerDir
		if l.Path != "" {
			layerRoot = filepath.Join(layerDir, l.Path)
		}

		result[name] = layerRoot
		fmt.Fprintf(w, "  %-20s → %s\n", name, layerRoot)
	}

	return result, nil
}

// ResolveLayerPaths returns the layer name → directory mapping for a project.
// Uses local overrides when set, otherwise checks the cache.
func ResolveLayerPaths(proj *yoestar.Project, projectRoot string) (map[string]string, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)

	for _, l := range proj.Layers {
		name := LayerName(l)

		if l.Local != "" {
			path := l.Local
			if !filepath.IsAbs(path) {
				path = filepath.Join(projectRoot, path)
			}
			result[name] = path
			continue
		}

		// Check cache
		layerDir := filepath.Join(cacheDir, name)
		if _, err := os.Stat(layerDir); err == nil {
			layerRoot := layerDir
			if l.Path != "" {
				layerRoot = filepath.Join(layerDir, l.Path)
			}
			result[name] = layerRoot
		}
		// If not cached, it will be missing — yoe layer sync is needed
	}

	return result, nil
}

// LayerName derives the layer name from a LayerRef.
// If Path is set, uses the last component of Path (e.g., "layers/recipes-core" → "recipes-core").
// Otherwise uses the last component of URL (e.g., "github.com/yoe/recipes-core" → "recipes-core").
func LayerName(l yoestar.LayerRef) string {
	if l.Path != "" {
		return filepath.Base(l.Path)
	}
	url := strings.TrimSuffix(l.URL, ".git")
	return filepath.Base(url)
}
