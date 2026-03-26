package repo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/YoeDistro/yoe-ng/internal/packaging"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// RepoDir returns the local package repository path for a project.
func RepoDir(proj *yoestar.Project, projectDir string) string {
	if proj != nil && proj.Repository.Path != "" {
		return proj.Repository.Path
	}
	return filepath.Join(projectDir, "build", "repo")
}

// Publish copies an .apk file to the local repository.
func Publish(apkPath, repoDir string) error {
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return err
	}

	name := filepath.Base(apkPath)
	dst := filepath.Join(repoDir, name)

	src, err := os.Open(apkPath)
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return err
	}

	return GenerateIndex(repoDir)
}

// List prints all packages in the local repository.
func List(repoDir string, w io.Writer) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(w, "No packages in repository")
			return nil
		}
		return err
	}

	var apks []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".apk") {
			apks = append(apks, e.Name())
		}
	}
	sort.Strings(apks)

	if len(apks) == 0 {
		fmt.Fprintln(w, "No packages in repository")
		return nil
	}

	fmt.Fprintf(w, "Repository: %s\n\n", repoDir)
	for _, name := range apks {
		info, _ := os.Stat(filepath.Join(repoDir, name))
		size := ""
		if info != nil {
			size = formatSize(info.Size())
		}
		fmt.Fprintf(w, "  %-40s %s\n", name, size)
	}
	fmt.Fprintf(w, "\n%d package(s)\n", len(apks))

	return nil
}

// Info shows details about a specific package in the repository.
func Info(repoDir, pkgName string, w io.Writer) error {
	// Find the package (allow partial name match)
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return err
	}

	var match string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), pkgName) && strings.HasSuffix(e.Name(), ".apk") {
			match = e.Name()
			break
		}
	}

	if match == "" {
		return fmt.Errorf("package %q not found in repository", pkgName)
	}

	apkPath := filepath.Join(repoDir, match)
	hash, err := packaging.APKHash(apkPath)
	if err != nil {
		return err
	}

	info, _ := os.Stat(apkPath)
	fmt.Fprintf(w, "Package:  %s\n", match)
	fmt.Fprintf(w, "SHA256:   %s\n", hash)
	if info != nil {
		fmt.Fprintf(w, "Size:     %s\n", formatSize(info.Size()))
	}

	return nil
}

// Remove deletes a package from the local repository.
func Remove(repoDir, pkgName string, w io.Writer) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return err
	}

	removed := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), pkgName) && strings.HasSuffix(e.Name(), ".apk") {
			path := filepath.Join(repoDir, e.Name())
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("removing %s: %w", e.Name(), err)
			}
			fmt.Fprintf(w, "Removed %s\n", e.Name())
			removed++
		}
	}

	if removed == 0 {
		return fmt.Errorf("package %q not found in repository", pkgName)
	}

	return nil
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1fM", float64(bytes)/(1024*1024))
}
