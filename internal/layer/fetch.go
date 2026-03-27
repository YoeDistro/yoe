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
		name := layerName(l.URL)

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

		result[name] = layerDir
		fmt.Fprintf(w, "  %-20s → %s\n", name, layerDir)
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
		name := layerName(l.URL)

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
			result[name] = layerDir
		}
		// If not cached, it will be missing — yoe layer sync is needed
	}

	return result, nil
}

// layerName extracts a short name from a layer URL.
// "github.com/yoe/recipes-core" → "recipes-core"
func layerName(url string) string {
	url = strings.TrimSuffix(url, ".git")
	return filepath.Base(url)
}
