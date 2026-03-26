package packaging

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// CreateAPK builds an .apk package from a recipe's $DESTDIR contents.
// Returns the path to the created .apk file.
func CreateAPK(recipe *yoestar.Recipe, destDir, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	apkName := fmt.Sprintf("%s-%s-r0.apk", recipe.Name, recipe.Version)
	apkPath := filepath.Join(outputDir, apkName)

	f, err := os.Create(apkPath)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", apkPath, err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Write .PKGINFO first
	pkginfo := generatePKGINFO(recipe, destDir)
	if err := writeFileToTar(tw, ".PKGINFO", []byte(pkginfo)); err != nil {
		return "", fmt.Errorf("writing .PKGINFO: %w", err)
	}

	// Walk destDir and add all files
	if err := addDirToTar(tw, destDir); err != nil {
		return "", fmt.Errorf("adding files to apk: %w", err)
	}

	return apkPath, nil
}

// generatePKGINFO creates the .PKGINFO metadata file content.
func generatePKGINFO(recipe *yoestar.Recipe, destDir string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("pkgname = %s\n", recipe.Name))
	b.WriteString(fmt.Sprintf("pkgver = %s-r0\n", recipe.Version))

	if recipe.Description != "" {
		b.WriteString(fmt.Sprintf("pkgdesc = %s\n", recipe.Description))
	}
	if recipe.License != "" {
		b.WriteString(fmt.Sprintf("license = %s\n", recipe.License))
	}

	b.WriteString(fmt.Sprintf("arch = %s\n", "x86_64")) // TODO: get from machine
	b.WriteString(fmt.Sprintf("builddate = %d\n", time.Now().Unix()))

	// Compute installed size
	var size int64
	filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	b.WriteString(fmt.Sprintf("size = %d\n", size))

	// Runtime dependencies
	for _, dep := range recipe.RuntimeDeps {
		b.WriteString(fmt.Sprintf("depend = %s\n", dep))
	}

	return b.String()
}

// addDirToTar walks a directory and adds all files to the tar archive.
func addDirToTar(tw *tar.Writer, baseDir string) error {
	// Collect paths first for deterministic ordering
	var paths []string
	if err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == baseDir {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(paths)

	for _, path := range paths {
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		info, err := os.Lstat(path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			header.Linkname = link
			header.Typeflag = tar.TypeSymlink
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() && info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, f); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

func writeFileToTar(tw *tar.Writer, name string, content []byte) error {
	header := &tar.Header{
		Name:    name,
		Size:    int64(len(content)),
		Mode:    0644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(content)
	return err
}

// APKHash computes the SHA256 hash of an .apk file.
func APKHash(apkPath string) (string, error) {
	f, err := os.Open(apkPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
