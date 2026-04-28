package device

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// Flash writes an image to a block device.
func Flash(proj *yoestar.Project, unitName, devicePath, projectDir string, dryRun bool, w io.Writer) error {
	unit, ok := proj.Units[unitName]
	if !ok {
		return fmt.Errorf("unit %q not found", unitName)
	}
	if unit.Class != "image" {
		return fmt.Errorf("unit %q is not an image (class=%q)", unitName, unit.Class)
	}

	// Resolve machine arch
	machine, ok := proj.Machines[proj.Defaults.Machine]
	if !ok {
		return fmt.Errorf("default machine %q not found", proj.Defaults.Machine)
	}

	// Find the built image
	imgPath := findImage(projectDir, machine.Name, unitName)
	if imgPath == "" {
		return fmt.Errorf("no built image found for %q — run yoe build %s first", unitName, unitName)
	}

	// Safety checks
	if err := validateDevice(devicePath); err != nil {
		return err
	}

	if dryRun {
		fmt.Fprintf(w, "Would flash %s → %s\n", imgPath, devicePath)
		return nil
	}

	fmt.Fprintf(w, "Flashing %s → %s\n", filepath.Base(imgPath), devicePath)
	fmt.Fprintln(w, "WARNING: This will overwrite all data on the device!")
	fmt.Fprint(w, "Continue? [y/N] ")

	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Fprintln(w, "Aborted")
		return nil
	}

	// Check if we have write access to the device
	if err := checkDeviceAccess(devicePath, w); err != nil {
		return err
	}

	// Use dd to write the image
	fmt.Fprintf(w, "Writing %s to %s...\n", filepath.Base(imgPath), devicePath)
	cmd := exec.Command("dd",
		"if="+imgPath,
		"of="+devicePath,
		"bs=4M",
		"conv=fsync",
		"status=progress",
	)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dd: %w", err)
	}

	fmt.Fprintln(w, "Flash complete")
	return nil
}

// checkDeviceAccess verifies write access to a block device. If permission
// is denied, offers to fix it with sudo chmod.
func checkDeviceAccess(devicePath string, w io.Writer) error {
	f, err := os.OpenFile(devicePath, os.O_WRONLY, 0)
	if err == nil {
		f.Close()
		return nil
	}

	if !os.IsPermission(err) {
		return fmt.Errorf("cannot open %s: %w", devicePath, err)
	}

	fmt.Fprintf(w, "Permission denied writing to %s\n", devicePath)
	fmt.Fprint(w, "Set write permissions with sudo? [y/N] ")

	var answer string
	fmt.Scanln(&answer)
	if strings.ToLower(answer) != "y" {
		return fmt.Errorf("no write permission on %s — run: sudo chmod 666 %s", devicePath, devicePath)
	}

	chmod := exec.Command("sudo", "chmod", "666", devicePath)
	chmod.Stdin = os.Stdin
	chmod.Stdout = w
	chmod.Stderr = os.Stderr
	if err := chmod.Run(); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	fmt.Fprintf(w, "Permissions set on %s\n", devicePath)
	return nil
}

func findImage(projectDir, scopeDir, unitName string) string {
	// Search both destdir (Starlark image class) and output (legacy)
	for _, subdir := range []string{"destdir", "output"} {
		dir := filepath.Join(projectDir, "build", unitName+"."+scopeDir, subdir)

		tarPath := filepath.Join(dir, unitName+".img.tar.gz")
		if _, err := os.Stat(tarPath); err == nil {
			return tarPath
		}

		imgPath := filepath.Join(dir, unitName+".img")
		if _, err := os.Stat(imgPath); err == nil {
			return imgPath
		}
	}

	return ""
}

func validateDevice(devicePath string) error {
	if devicePath == "" {
		return fmt.Errorf("device path required")
	}

	info, err := os.Stat(devicePath)
	if err != nil {
		return fmt.Errorf("device %s: %w", devicePath, err)
	}

	if info.Mode()&os.ModeDevice == 0 {
		return fmt.Errorf("%s is not a block device", devicePath)
	}

	resolved, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", devicePath, err)
	}
	targetDisk := parentDisk(resolved)

	mountsData, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return fmt.Errorf("read /proc/mounts: %w", err)
	}
	for _, sysDisk := range systemDisks(string(mountsData)) {
		if sysDisk == targetDisk {
			return fmt.Errorf("refusing to write to %s: hosts the running system", devicePath)
		}
	}

	return nil
}

// parentDisk returns the whole-disk device for a partition (e.g. /dev/sda1
// → /dev/sda, /dev/nvme0n1p2 → /dev/nvme0n1). If devicePath is not a
// partition, or its sysfs entry can't be read, returns devicePath unchanged.
func parentDisk(devicePath string) string {
	name := filepath.Base(devicePath)
	sysPath := "/sys/class/block/" + name
	if _, err := os.Stat(filepath.Join(sysPath, "partition")); err != nil {
		return devicePath
	}
	target, err := os.Readlink(sysPath)
	if err != nil {
		return devicePath
	}
	return "/dev/" + filepath.Base(filepath.Dir(target))
}

// systemDisks returns the set of whole-disk devices that host critical
// system mountpoints. Walks /sys/class/block/<name>/slaves to resolve
// dm-crypt, LVM, and md devices to their underlying physical disks.
func systemDisks(mountsContent string) []string {
	critical := map[string]bool{
		"/":         true,
		"/boot":     true,
		"/boot/efi": true,
		"/usr":      true,
	}
	seen := map[string]bool{}
	var disks []string
	for _, line := range strings.Split(mountsContent, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !critical[fields[1]] {
			continue
		}
		src := fields[0]
		if !strings.HasPrefix(src, "/dev/") {
			continue
		}
		resolved, err := filepath.EvalSymlinks(src)
		if err != nil {
			continue
		}
		for _, base := range underlyingDevices(resolved) {
			disk := parentDisk(base)
			if !seen[disk] {
				seen[disk] = true
				disks = append(disks, disk)
			}
		}
	}
	return disks
}

// underlyingDevices recurses through /sys/class/block/<name>/slaves to find
// the physical block devices backing a dm-/md device. For a leaf device
// with no slaves, returns devicePath unchanged.
func underlyingDevices(devicePath string) []string {
	slavesDir := filepath.Join("/sys/class/block", filepath.Base(devicePath), "slaves")
	entries, err := os.ReadDir(slavesDir)
	if err != nil || len(entries) == 0 {
		return []string{devicePath}
	}
	var out []string
	for _, e := range entries {
		out = append(out, underlyingDevices("/dev/"+e.Name())...)
	}
	return out
}
