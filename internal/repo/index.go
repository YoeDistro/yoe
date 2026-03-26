package repo

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// GenerateIndex scans repoDir for .apk files and produces an
// APKINDEX.tar.gz that apk(8) can use for dependency resolution.
func GenerateIndex(repoDir string) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return fmt.Errorf("reading repo dir: %w", err)
	}

	var apks []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".apk") {
			apks = append(apks, e.Name())
		}
	}
	sort.Strings(apks)

	if len(apks) == 0 {
		return nil // nothing to index
	}

	// Build APKINDEX content
	var buf strings.Builder
	for i, name := range apks {
		apkPath := filepath.Join(repoDir, name)

		info, err := os.Stat(apkPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", name, err)
		}

		hash, err := sha256sum(apkPath)
		if err != nil {
			return fmt.Errorf("hashing %s: %w", name, err)
		}

		pkgName, version := parseAPKFilename(name)
		installedSize := extractInstalledSize(apkPath)
		description := extractDescription(apkPath)

		// C: checksum line (Q1 prefix = SHA-1 in Alpine, but we use SHA256 hex)
		fmt.Fprintf(&buf, "C:Q1%s\n", hash)
		fmt.Fprintf(&buf, "P:%s\n", pkgName)
		fmt.Fprintf(&buf, "V:%s\n", version)
		fmt.Fprintf(&buf, "S:%d\n", info.Size())
		fmt.Fprintf(&buf, "I:%d\n", installedSize)
		fmt.Fprintf(&buf, "T:%s\n", description)
		fmt.Fprintf(&buf, "A:x86_64\n")
		if i < len(apks)-1 {
			buf.WriteString("\n")
		}
	}

	// Write APKINDEX.tar.gz
	indexPath := filepath.Join(repoDir, "APKINDEX.tar.gz")
	f, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("creating APKINDEX.tar.gz: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	content := []byte(buf.String())
	hdr := &tar.Header{
		Name:    "APKINDEX",
		Size:    int64(len(content)),
		Mode:    0644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("writing tar header: %w", err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("writing tar content: %w", err)
	}

	return nil
}

// parseAPKFilename parses "name-version-rN.apk" into (name, version-rN).
// It strips .apk, finds the last -rN suffix, then splits name from version
// at the last dash before a digit.
func parseAPKFilename(filename string) (name, version string) {
	// Strip .apk extension
	s := strings.TrimSuffix(filename, ".apk")

	// Find the last "-rN" revision suffix by scanning backwards.
	// The revision is always at the end: "-r" followed by digits.
	revIdx := strings.LastIndex(s, "-r")
	if revIdx < 0 {
		// No revision suffix; treat the whole thing as name
		return s, ""
	}
	// Verify everything after "-r" is digits
	rev := s[revIdx+2:]
	allDigits := len(rev) > 0
	for _, c := range rev {
		if !unicode.IsDigit(c) {
			allDigits = false
			break
		}
	}
	if !allDigits {
		return s, ""
	}

	// Now we have e.g. "hello-1.0.0" with revision "r0"
	nameVer := s[:revIdx] // "hello-1.0.0"
	revision := s[revIdx:]  // "-r0"

	// Find version separator: scan backwards for last dash before a digit
	verIdx := -1
	for i := len(nameVer) - 1; i >= 0; i-- {
		if nameVer[i] == '-' {
			// Check if next char is a digit (start of version)
			if i+1 < len(nameVer) && unicode.IsDigit(rune(nameVer[i+1])) {
				verIdx = i
				break
			}
		}
	}

	if verIdx < 0 {
		return nameVer, revision[1:] // strip leading dash from revision
	}

	name = nameVer[:verIdx]
	version = nameVer[verIdx+1:] + revision

	return name, version
}

// sha256sum returns the hex-encoded SHA256 of a file.
func sha256sum(path string) (string, error) {
	f, err := os.Open(path)
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

// extractInstalledSize reads the .PKGINFO from an .apk and returns the
// "size" value. Returns 0 if it cannot be determined.
func extractInstalledSize(apkPath string) int64 {
	val := extractPKGINFOField(apkPath, "size")
	if val == "" {
		return 0
	}
	var size int64
	fmt.Sscanf(val, "%d", &size)
	return size
}

// extractDescription reads the .PKGINFO from an .apk and returns the
// "pkgdesc" value.
func extractDescription(apkPath string) string {
	return extractPKGINFOField(apkPath, "pkgdesc")
}

// extractPKGINFOField opens an .apk (gzip'd tar), finds .PKGINFO, and
// returns the value for the given key. Returns "" on any failure.
func extractPKGINFOField(apkPath, key string) string {
	f, err := os.Open(apkPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return ""
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return ""
		}
		if hdr.Name == ".PKGINFO" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return ""
			}
			prefix := key + " = "
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, prefix) {
					return strings.TrimPrefix(line, prefix)
				}
			}
			return ""
		}
	}
}
