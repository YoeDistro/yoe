package source

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// Prepare sets up the build source directory for a unit:
// 1. Fetches source (from cache or network)
// 2. Extracts into build/<unit>/src/ as a git repo with "upstream" tag
// 3. Applies patches from the unit as git commits
//
// If the source directory already exists with local commits beyond upstream,
// it is left untouched (yoe dev workflow).
func Prepare(projectDir string, unit *yoestar.Unit) (string, error) {
	srcDir := filepath.Join(projectDir, "build", unit.Name, "src")

	// If source dir exists and has local commits, don't touch it (dev mode)
	if hasLocalCommits(srcDir) {
		fmt.Printf("Using local source for %s (has commits beyond upstream)\n", unit.Name)
		return srcDir, nil
	}

	if unit.Source == "" {
		return "", fmt.Errorf("unit %q has no source", unit.Name)
	}

	// Fetch source into cache
	cachedPath, err := Fetch(unit)
	if err != nil {
		return "", err
	}

	// Remove old source dir and recreate
	os.RemoveAll(srcDir)
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return "", err
	}

	// Extract or checkout
	if isGitURL(unit.Source) {
		if err := checkoutGit(cachedPath, srcDir, unit); err != nil {
			return "", err
		}
		// Git source is already a repo — just tag current HEAD as upstream
		if err := tagUpstream(srcDir); err != nil {
			return "", err
		}
	} else {
		if err := extractTarball(cachedPath, srcDir); err != nil {
			return "", err
		}
		// Tarball needs git init + commit + tag
		if err := initGitRepo(srcDir); err != nil {
			return "", err
		}
	}

	// Apply patches
	if err := applyPatches(projectDir, srcDir, unit); err != nil {
		return "", err
	}

	return srcDir, nil
}

// hasLocalCommits checks if a source directory is a git repo with commits
// beyond the upstream tag.
func hasLocalCommits(srcDir string) bool {
	gitDir := filepath.Join(srcDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return false
	}

	cmd := exec.Command("git", "rev-list", "--count", "upstream..HEAD")
	cmd.Dir = srcDir
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	count := strings.TrimSpace(string(out))
	return count != "0"
}

func checkoutGit(barePath, srcDir string, unit *yoestar.Unit) error {
	// Determine ref to checkout
	ref := "HEAD"
	if unit.Tag != "" {
		ref = unit.Tag
	} else if unit.Branch != "" {
		ref = unit.Branch
	}

	// Clone from bare cache into srcDir
	cmd := exec.Command("git", "clone", "--shared", barePath, srcDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %s\n%s", err, out)
	}

	// Checkout the right ref
	cmd = exec.Command("git", "checkout", ref)
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout %s: %s\n%s", ref, err, out)
	}

	return nil
}

func extractTarball(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader = f

	// Detect compression
	switch {
	case strings.HasSuffix(tarPath, ".gz") || strings.HasSuffix(tarPath, ".tgz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close()
		reader = gz
	case strings.HasSuffix(tarPath, ".bz2"):
		reader = bzip2.NewReader(f)
	case strings.HasSuffix(tarPath, ".xz"):
		// Go stdlib doesn't have xz; shell out
		return extractWithTar(tarPath, destDir)
	}

	tr := tar.NewReader(reader)
	// Strip the first path component (most tarballs have a top-level dir)
	stripPrefix := ""

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tarball: %w", err)
		}

		// Detect top-level directory to strip
		if stripPrefix == "" {
			parts := strings.SplitN(hdr.Name, "/", 2)
			if len(parts) > 1 {
				stripPrefix = parts[0] + "/"
			}
		}

		name := strings.TrimPrefix(hdr.Name, stripPrefix)
		if name == "" || name == "." {
			continue
		}

		target := filepath.Join(destDir, name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, os.FileMode(hdr.Mode))
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755)
			os.Symlink(hdr.Linkname, target)
		}
	}

	return nil
}

func extractWithTar(tarPath, destDir string) error {
	cmd := exec.Command("tar", "xf", tarPath, "--strip-components=1", "-C", destDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar extract: %s\n%s", err, out)
	}
	return nil
}

// tagUpstream tags the current HEAD as "upstream" in an existing git repo.
// Used for git-sourced recipes where the checkout is already a git repo.
func tagUpstream(srcDir string) error {
	// Ensure we're on a branch (shallow clones may be detached)
	branchCmd := exec.Command("git", "checkout", "-b", "yoe-work")
	branchCmd.Dir = srcDir
	branchCmd.Run() // ignore error if branch already exists
	cmd := exec.Command("git", "tag", "-f", "upstream")
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git tag upstream: %s\n%s", err, out)
	}
	return nil
}

func initGitRepo(srcDir string) error {
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "yoe@yoe-ng.local"},
		{"git", "config", "user.name", "yoe-ng"},
		{"git", "add", "-A"},
		{"git", "commit", "-m", "upstream source"},
		{"git", "tag", "upstream"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = srcDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s\n%s", strings.Join(args, " "), err, out)
		}
	}

	return nil
}

func applyPatches(projectDir, srcDir string, unit *yoestar.Unit) error {
	for _, patchFile := range unit.Patches {
		patchPath := filepath.Join(projectDir, patchFile)
		if _, err := os.Stat(patchPath); os.IsNotExist(err) {
			return fmt.Errorf("patch file not found: %s", patchFile)
		}

		// Apply with git am (preserves commit message from patch)
		cmd := exec.Command("git", "am", "--3way", patchPath)
		cmd.Dir = srcDir
		if out, err := cmd.CombinedOutput(); err != nil {
			// Fallback to git apply
			cmd = exec.Command("git", "apply", patchPath)
			cmd.Dir = srcDir
			if out2, err2 := cmd.CombinedOutput(); err2 != nil {
				return fmt.Errorf("applying %s: git am: %s\ngit apply: %s\n%s\n%s",
					patchFile, err, err2, out, out2)
			}
			// Commit the applied patch
			commitMsg := fmt.Sprintf("patch: %s", filepath.Base(patchFile))
			cmds := [][]string{
				{"git", "add", "-A"},
				{"git", "commit", "-m", commitMsg},
			}
			for _, args := range cmds {
				c := exec.Command(args[0], args[1:]...)
				c.Dir = srcDir
				if out, err := c.CombinedOutput(); err != nil {
					return fmt.Errorf("%s: %s\n%s", strings.Join(args, " "), err, out)
				}
			}
		}
	}

	return nil
}
