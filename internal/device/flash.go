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

	// Use dd to write the image
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

	// Check it exists
	info, err := os.Stat(devicePath)
	if err != nil {
		return fmt.Errorf("device %s: %w", devicePath, err)
	}

	// Must be a block device
	if info.Mode()&os.ModeDevice == 0 {
		return fmt.Errorf("%s is not a block device", devicePath)
	}

	// Refuse to write to common system disk paths
	dangerous := []string{"/dev/sda", "/dev/nvme0n1", "/dev/vda"}
	for _, d := range dangerous {
		if devicePath == d {
			return fmt.Errorf("refusing to write to %s (looks like a system disk)", devicePath)
		}
	}

	return nil
}
