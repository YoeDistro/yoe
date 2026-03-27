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

	// Step 1: Install packages into rootfs — resolve deps so libraries
	// are included automatically (e.g., openssh pulls in openssl, zlib).
	repoDir := repo.RepoBaseDir(proj, projectDir)
	allPackages := resolvePackageDeps(recipe.Packages, proj)
	if err := installPackages(rootfs, repoDir, allPackages, w); err != nil {
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
	if err := generateImage(rootfs, imgPath, recipe, projectDir, w); err != nil {
		return fmt.Errorf("generating image: %w", err)
	}

	fmt.Fprintf(w, "  → %s\n", imgPath)
	return nil
}

// resolvePackageDeps expands a list of package names to include all transitive
// dependencies (both build and runtime). The returned list is in dependency
// order (deps before dependents), with image-class recipes excluded.
func resolvePackageDeps(packages []string, proj *yoestar.Project) []string {
	seen := make(map[string]bool)
	var result []string

	var walk func(name string)
	walk = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true

		if recipe, ok := proj.Recipes[name]; ok {
			// Skip image recipes — they aren't installable packages
			if recipe.Class == "image" {
				return
			}
			for _, dep := range recipe.Deps {
				walk(dep)
			}
			for _, dep := range recipe.RuntimeDeps {
				walk(dep)
			}
		}
		result = append(result, name)
	}

	for _, pkg := range packages {
		walk(pkg)
	}
	return result
}

func installPackages(rootfs, repoDir string, packages []string, w io.Writer) error {
	if len(packages) == 0 {
		fmt.Fprintln(w, "  (no packages to install)")
		return nil
	}

	fmt.Fprintf(w, "  Installing %d packages into rootfs...\n", len(packages))

	// Install packages by extracting .apk files directly into the rootfs.
	// Our packages are single-stream gzip'd tars with .PKGINFO + files.
	// tar is available on the host — no container needed.
	absRepo, _ := filepath.Abs(repoDir)

	for _, pkg := range packages {
		apkFile := findAPK(absRepo, pkg)
		if apkFile == "" {
			return fmt.Errorf("package %q not found in %s", pkg, absRepo)
		}

		fmt.Fprintf(w, "    %s\n", filepath.Base(apkFile))

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

	// Create essential directories for virtual filesystem mount points.
	// mkfs.ext4 -d only copies non-empty directories, so we add a
	// .keep file to ensure they exist in the image.
	for _, dir := range []string{"proc", "sys", "dev", "tmp", "run"} {
		dirPath := filepath.Join(rootfs, dir)
		os.MkdirAll(dirPath, 0755)
		os.WriteFile(filepath.Join(dirPath, ".keep"), nil, 0644)
	}

	// Install boot configuration (extlinux for QEMU serial console)
	bootDir := filepath.Join(rootfs, "boot", "extlinux")
	os.MkdirAll(bootDir, 0755)
	extlinuxConf := `DEFAULT yoe
LABEL yoe
    LINUX /boot/vmlinuz
    APPEND console=ttyS0 root=/dev/vda1 rw devtmpfs.mount=1
`
	os.WriteFile(filepath.Join(bootDir, "extlinux.conf"), []byte(extlinuxConf), 0644)
	fmt.Fprintln(w, "  Installed boot configuration (extlinux)")

	// Install minimal inittab for busybox init
	inittab := `::sysinit:/bin/mount -t proc proc /proc
::sysinit:/bin/mount -t sysfs sys /sys
::sysinit:/bin/hostname -F /etc/hostname
ttyS0::respawn:/bin/sh
::ctrlaltdel:/sbin/reboot
::shutdown:/bin/umount -a -r
`
	os.WriteFile(filepath.Join(rootfs, "etc", "inittab"), []byte(inittab), 0644)

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

func generateImage(rootfs, imgPath string, recipe *yoestar.Recipe, projectDir string, w io.Writer) error {
	fmt.Fprintln(w, "  Generating disk image...")
	return GenerateDiskImage(rootfs, imgPath, recipe, projectDir, w)
}
