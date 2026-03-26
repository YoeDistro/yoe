package image

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/YoeDistro/yoe-ng/internal/repo"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// Assemble creates a root filesystem image from an image recipe.
func Assemble(recipe *yoestar.Recipe, proj *yoestar.Project, projectDir, outputDir string, w io.Writer) error {
	rootfs := filepath.Join(outputDir, "rootfs")
	os.RemoveAll(rootfs)
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		return fmt.Errorf("creating rootfs dir: %w", err)
	}

	// Step 1: Install packages into rootfs via apk
	repoDir := repo.RepoBaseDir(proj, projectDir)
	if err := installPackages(rootfs, repoDir, recipe.Packages, w); err != nil {
		return fmt.Errorf("installing packages: %w", err)
	}

	// Step 2: Apply configuration (hostname, timezone, locale, services)
	if err := applyConfig(rootfs, recipe, w); err != nil {
		return fmt.Errorf("applying config: %w", err)
	}

	// Step 3: Apply overlays
	overlayDir := filepath.Join(projectDir, "overlays")
	if _, err := os.Stat(overlayDir); err == nil {
		if err := applyOverlays(rootfs, overlayDir, w); err != nil {
			return fmt.Errorf("applying overlays: %w", err)
		}
	}

	// Step 4: Generate disk image
	imgPath := filepath.Join(outputDir, recipe.Name+".img")
	if err := generateImage(rootfs, imgPath, recipe, w); err != nil {
		return fmt.Errorf("generating image: %w", err)
	}

	fmt.Fprintf(w, "  → %s\n", imgPath)
	return nil
}

func installPackages(rootfs, repoDir string, packages []string, w io.Writer) error {
	if len(packages) == 0 {
		fmt.Fprintln(w, "  (no packages to install)")
		return nil
	}

	fmt.Fprintf(w, "  Installing %d packages into rootfs...\n", len(packages))

	// Check if apk is available
	if _, err := exec.LookPath("apk"); err != nil {
		// Fallback: just create the directory structure
		fmt.Fprintln(w, "  (apk not available — creating minimal rootfs structure)")
		for _, dir := range []string{"usr/bin", "usr/lib", "etc", "var", "tmp"} {
			os.MkdirAll(filepath.Join(rootfs, dir), 0755)
		}

		// Copy .apk files from repo into rootfs as a manifest
		for _, pkg := range packages {
			fmt.Fprintf(w, "    %s\n", pkg)
		}
		// Write package list for reference
		os.WriteFile(filepath.Join(rootfs, "etc", "yoe-packages"),
			[]byte(strings.Join(packages, "\n")+"\n"), 0644)
		return nil
	}

	// Install packages by extracting .apk files directly into the rootfs.
	// Our packages are single-stream gzip'd tars with .PKGINFO + files.
	// This avoids apk signature verification issues with our unsigned packages.
	// When we add package signing, we can switch to using apk directly.
	absRepo, _ := filepath.Abs(repoDir)

	for _, pkg := range packages {
		// Find the .apk file for this package
		apkFile := findAPK(absRepo, pkg)
		if apkFile == "" {
			return fmt.Errorf("package %q not found in %s", pkg, absRepo)
		}

		fmt.Fprintf(w, "    %s\n", filepath.Base(apkFile))

		// Extract tar.gz into rootfs, skipping .PKGINFO
		cmd := exec.Command("tar", "xzf", apkFile, "-C", rootfs, "--exclude=.PKGINFO")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("extracting %s: %s\n%s", pkg, err, out)
		}
	}

	return nil
}

// findAPK finds the .apk file for a package name in the repo directory.
func findAPK(repoDir, pkgName string) string {
	// Check arch subdirectory first, then root
	for _, dir := range []string{repoDir, filepath.Join(repoDir, "x86_64")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), pkgName+"-") && strings.HasSuffix(e.Name(), ".apk") {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return ""
}

func applyConfig(rootfs string, recipe *yoestar.Recipe, w io.Writer) error {
	if recipe.Hostname != "" {
		fmt.Fprintf(w, "  Setting hostname: %s\n", recipe.Hostname)
		os.MkdirAll(filepath.Join(rootfs, "etc"), 0755)
		os.WriteFile(filepath.Join(rootfs, "etc", "hostname"),
			[]byte(recipe.Hostname+"\n"), 0644)
	}

	if recipe.Timezone != "" {
		fmt.Fprintf(w, "  Setting timezone: %s\n", recipe.Timezone)
		os.MkdirAll(filepath.Join(rootfs, "etc"), 0755)
		// Create symlink for localtime
		localtime := filepath.Join(rootfs, "etc", "localtime")
		os.Remove(localtime)
		os.Symlink("/usr/share/zoneinfo/"+recipe.Timezone, localtime)
	}

	if recipe.Locale != "" {
		os.MkdirAll(filepath.Join(rootfs, "etc"), 0755)
		os.WriteFile(filepath.Join(rootfs, "etc", "locale.conf"),
			[]byte("LANG="+recipe.Locale+"\n"), 0644)
	}

	// Enable systemd services
	for _, svc := range recipe.Services {
		fmt.Fprintf(w, "  Enabling service: %s\n", svc)
		svcDir := filepath.Join(rootfs, "etc", "systemd", "system", "multi-user.target.wants")
		os.MkdirAll(svcDir, 0755)
		link := filepath.Join(svcDir, svc+".service")
		target := "/usr/lib/systemd/system/" + svc + ".service"
		os.Symlink(target, link)
	}

	// Install boot configuration (extlinux for QEMU serial console)
	bootDir := filepath.Join(rootfs, "boot", "extlinux")
	os.MkdirAll(bootDir, 0755)
	extlinuxConf := `DEFAULT yoe
LABEL yoe
    LINUX /boot/vmlinuz
    APPEND console=ttyS0 root=/dev/vda1 rw init=/bin/sh
`
	os.WriteFile(filepath.Join(bootDir, "extlinux.conf"), []byte(extlinuxConf), 0644)
	fmt.Fprintln(w, "  Installed boot configuration (extlinux)")

	return nil
}

func applyOverlays(rootfs, overlayDir string, w io.Writer) error {
	return filepath.WalkDir(overlayDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == overlayDir {
			return nil
		}

		rel, _ := filepath.Rel(overlayDir, path)
		dst := filepath.Join(rootfs, rel)

		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}

		fmt.Fprintf(w, "  Overlay: %s\n", rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(dst), 0755)
		return os.WriteFile(dst, data, 0644)
	})
}

func generateImage(rootfs, imgPath string, recipe *yoestar.Recipe, w io.Writer) error {
	fmt.Fprintln(w, "  Generating disk image...")
	return GenerateDiskImage(rootfs, imgPath, recipe, w)
}
