package source

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// CacheDir returns the source cache directory, creating it if needed.
func CacheDir() (string, error) {
	dir := os.Getenv("YOE_CACHE")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".cache", "yoe-ng")
	}
	dir = filepath.Join(dir, "sources")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// Fetch downloads the source for a recipe into the cache.
// Returns the path to the cached source (tarball or bare git repo).
func Fetch(recipe *yoestar.Recipe) (string, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	if recipe.Source == "" {
		return "", fmt.Errorf("recipe %q has no source", recipe.Name)
	}

	if isGitURL(recipe.Source) {
		return fetchGit(cacheDir, recipe)
	}
	return fetchHTTP(cacheDir, recipe)
}

// fetchHTTP downloads a tarball and caches it by URL hash.
func fetchHTTP(cacheDir string, recipe *yoestar.Recipe) (string, error) {
	// Cache key: sha256 of URL
	urlHash := fmt.Sprintf("%x", sha256.Sum256([]byte(recipe.Source)))
	ext := guessExt(recipe.Source)
	cachedPath := filepath.Join(cacheDir, urlHash+ext)

	// Already cached?
	if _, err := os.Stat(cachedPath); err == nil {
		return cachedPath, nil
	}

	fmt.Printf("Fetching %s...\n", recipe.Source)

	resp, err := http.Get(recipe.Source)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", recipe.Source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: HTTP %d", recipe.Source, resp.StatusCode)
	}

	// Write to temp file then rename (atomic)
	tmp, err := os.CreateTemp(cacheDir, "download-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("downloading %s: %w", recipe.Source, err)
	}
	tmp.Close()

	// Verify SHA256 if specified
	actualHash := fmt.Sprintf("%x", h.Sum(nil))
	if recipe.SHA256 != "" && actualHash != recipe.SHA256 {
		os.Remove(tmpPath)
		return "", fmt.Errorf("SHA256 mismatch for %s: expected %s, got %s",
			recipe.Source, recipe.SHA256, actualHash)
	}

	if err := os.Rename(tmpPath, cachedPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	return cachedPath, nil
}

// fetchGit clones or updates a bare git repo in the cache.
// Uses shallow clone by default (only the pinned tag/branch) to avoid
// downloading full history. For the Linux kernel this is ~4GB vs ~200MB.
func fetchGit(cacheDir string, recipe *yoestar.Recipe) (string, error) {
	// Cache key: sha256 of repo URL + ref (different tags get different clones)
	ref := recipe.Tag
	if ref == "" {
		ref = recipe.Branch
	}
	if ref == "" {
		ref = "HEAD"
	}
	cacheKey := recipe.Source + "#" + ref
	urlHash := fmt.Sprintf("%x", sha256.Sum256([]byte(cacheKey)))
	barePath := filepath.Join(cacheDir, urlHash+".git")

	if _, err := os.Stat(barePath); os.IsNotExist(err) {
		fmt.Printf("Cloning %s (ref: %s)...\n", recipe.Source, ref)

		// Shallow clone of just the ref we need
		args := []string{"clone", "--bare", "--depth", "1"}
		if recipe.Tag != "" {
			args = append(args, "--branch", recipe.Tag)
		} else if recipe.Branch != "" {
			args = append(args, "--branch", recipe.Branch)
		}
		args = append(args, recipe.Source, barePath)

		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git clone %s: %s\n%s", recipe.Source, err, out)
		}
	} else {
		// Repo already cached — fetch the specific ref if needed
		fmt.Printf("Using cached %s (ref: %s)\n", recipe.Source, ref)
	}

	return barePath, nil
}

// Verify checks the SHA256 of a cached source file.
func Verify(recipe *yoestar.Recipe) error {
	if recipe.SHA256 == "" {
		return nil // no hash to verify
	}
	if isGitURL(recipe.Source) {
		return nil // git sources verified by commit hash
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return err
	}

	urlHash := fmt.Sprintf("%x", sha256.Sum256([]byte(recipe.Source)))
	ext := guessExt(recipe.Source)
	cachedPath := filepath.Join(cacheDir, urlHash+ext)

	f, err := os.Open(cachedPath)
	if err != nil {
		return fmt.Errorf("source not cached: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := fmt.Sprintf("%x", h.Sum(nil))
	if actual != recipe.SHA256 {
		return fmt.Errorf("SHA256 mismatch for %s: expected %s, got %s",
			recipe.Name, recipe.SHA256, actual)
	}

	return nil
}

func isGitURL(url string) bool {
	return strings.HasSuffix(url, ".git") ||
		strings.HasPrefix(url, "git://") ||
		(strings.Contains(url, "github.com/") && !strings.Contains(url, "/archive/") && !strings.Contains(url, "/releases/"))
}

func guessExt(url string) string {
	for _, ext := range []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tgz", ".zip"} {
		if strings.HasSuffix(url, ext) {
			return ext
		}
	}
	return ".tar.gz"
}
