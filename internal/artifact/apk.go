package artifact

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
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

// CreateAPK builds an .apk package from a unit's $DESTDIR contents.
//
// Alpine .apk files are concatenated gzip streams:
//   - Stream 1 (optional): signature block (.SIGN.RSA.*)
//   - Stream 2: control block (.PKGINFO + checksums)
//   - Stream 3: data block (actual files)
//
// For unsigned packages, we write only streams 2 and 3.
// apk with --allow-untrusted accepts this format.
func CreateAPK(unit *yoestar.Unit, destDir, outputDir, scopeDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	apkName := fmt.Sprintf("%s-%s-r0.%s.apk", unit.Name, unit.Version, scopeDir)
	apkPath := filepath.Join(outputDir, apkName)

	// Write a single gzip stream containing .PKGINFO followed by all files.
	// This is the simplest format that apk accepts with --allow-untrusted.
	// The multi-stream format (signature + control + data) requires proper
	// signing infrastructure — we'll add that later.
	f, err := os.Create(apkPath)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", apkPath, err)
	}

	pkginfo := generatePKGINFO(unit, destDir, "", scopeDir)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Write .PKGINFO first
	hdr := &tar.Header{Name: ".PKGINFO", Size: int64(len(pkginfo)), Mode: 0644, ModTime: time.Now()}
	tw.WriteHeader(hdr)
	tw.Write([]byte(pkginfo))

	// Write all files from destDir
	addDirToTar(tw, destDir)

	tw.Close()
	gw.Close()
	f.Close()

	return apkPath, nil
}

// addDirToTar walks a directory and adds all files to the tar archive.
func addDirToTar(tw *tar.Writer, baseDir string) error {
	var paths []string
	filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == baseDir {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	sort.Strings(paths)

	for _, path := range paths {
		rel, _ := filepath.Rel(baseDir, path)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		header, _ := tar.FileInfoHeader(info, "")
		header.Name = rel
		if info.Mode()&os.ModeSymlink != 0 {
			link, _ := os.Readlink(path)
			header.Linkname = link
			header.Typeflag = tar.TypeSymlink
		}
		tw.WriteHeader(header)
		if info.Mode().IsRegular() {
			f, _ := os.Open(path)
			io.Copy(tw, f)
			f.Close()
		}
	}
	return nil
}

// buildDataTar creates an uncompressed tar archive of the destDir contents.
func buildDataTar(destDir string) ([]byte, error) {
	var paths []string
	if err := filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == destDir {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(paths)

	// Write to a temp file (packages can be large)
	tmp, err := os.CreateTemp("", "yoe-data-*.tar")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	tw := tar.NewWriter(tmp)
	for _, path := range paths {
		rel, _ := filepath.Rel(destDir, path)
		info, err := os.Lstat(path)
		if err != nil {
			tmp.Close()
			return nil, err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			tmp.Close()
			return nil, err
		}
		header.Name = rel

		if info.Mode()&os.ModeSymlink != 0 {
			link, _ := os.Readlink(path)
			header.Linkname = link
			header.Typeflag = tar.TypeSymlink
		}

		if err := tw.WriteHeader(header); err != nil {
			tmp.Close()
			return nil, err
		}

		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				tmp.Close()
				return nil, err
			}
			io.Copy(tw, f)
			f.Close()
		}
	}
	tw.Close()
	tmp.Close()

	return os.ReadFile(tmpName)
}

// writeGzipTar writes a single gzip stream containing a tar with the given files.
func writeGzipTar(w io.Writer, files map[string][]byte) error {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	// Sort keys for determinism
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		content := files[name]
		header := &tar.Header{
			Name:    name,
			Size:    int64(len(content)),
			Mode:    0644,
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write(content); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return gw.Close()
}

// generatePKGINFO creates the .PKGINFO metadata file content.
func generatePKGINFO(unit *yoestar.Unit, destDir, dataHashHex, arch string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "pkgname = %s\n", unit.Name)
	fmt.Fprintf(&b, "pkgver = %s-r0\n", unit.Version)

	if unit.Description != "" {
		fmt.Fprintf(&b, "pkgdesc = %s\n", unit.Description)
	}
	if unit.License != "" {
		fmt.Fprintf(&b, "license = %s\n", unit.License)
	}

	fmt.Fprintf(&b, "arch = %s\n", arch)
	fmt.Fprintf(&b, "builddate = %d\n", time.Now().Unix())

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
	fmt.Fprintf(&b, "size = %d\n", size)

	// Data hash (SHA256 of the uncompressed data tar)
	if dataHashHex != "" {
		fmt.Fprintf(&b, "datahash = %s\n", dataHashHex)
	}

	// Runtime dependencies
	for _, dep := range unit.RuntimeDeps {
		fmt.Fprintf(&b, "depend = %s\n", dep)
	}

	return b.String()
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

// APKSha1 computes the SHA1 hash of an .apk file (for APKINDEX C: field).
func APKSha1(apkPath string) ([]byte, error) {
	f, err := os.Open(apkPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}
