package image

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// GenerateDiskImage creates a partitioned disk image from a rootfs directory.
func GenerateDiskImage(rootfs, imgPath string, recipe *yoestar.Recipe, w io.Writer) error {
	totalMB := 0
	for _, p := range recipe.Partitions {
		size := parseSizeMB(p.Size)
		if size == 0 {
			size = 512
		}
		totalMB += size
	}
	if totalMB == 0 {
		totalMB = 512
	}

	fmt.Fprintf(w, "  Creating %dMB disk image...\n", totalMB)

	if err := createSparseImage(imgPath, totalMB); err != nil {
		return fmt.Errorf("creating image: %w", err)
	}

	if err := partitionImage(imgPath, recipe.Partitions, w); err != nil {
		return fmt.Errorf("partitioning: %w", err)
	}

	if err := populateImage(imgPath, rootfs, recipe, w); err != nil {
		return fmt.Errorf("populating: %w", err)
	}

	return nil
}

func createSparseImage(path string, sizeMB int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Truncate(int64(sizeMB) * 1024 * 1024)
}

func partitionImage(imgPath string, partitions []yoestar.Partition, w io.Writer) error {
	if len(partitions) == 0 {
		partitions = []yoestar.Partition{
			{Label: "rootfs", Type: "ext4", Size: "fill", Root: true},
		}
	}

	script := "label: gpt\n"
	for _, p := range partitions {
		size := ""
		sizeMB := parseSizeMB(p.Size)
		if sizeMB > 0 {
			size = fmt.Sprintf("size=%dM, ", sizeMB)
		}
		ptype := "linux"
		if p.Type == "vfat" {
			ptype = "uefi"
		}
		script += fmt.Sprintf("%stype=%s, name=%s\n", size, ptype, p.Label)
	}

	fmt.Fprintf(w, "  Partitioning (GPT)...\n")

	// Check if sfdisk is available
	if _, err := exec.LookPath("sfdisk"); err != nil {
		fmt.Fprintf(w, "  (sfdisk not available — creating rootfs.tar.gz fallback)\n")
		return nil // fall through to tar fallback
	}

	cmd := exec.Command("sfdisk", imgPath)
	cmd.Stdin = strings.NewReader(script)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func populateImage(imgPath, rootfs string, recipe *yoestar.Recipe, w io.Writer) error {
	// Try loop device approach (requires root or user namespaces)
	out, err := exec.Command("losetup", "--find", "--show", "--partscan", imgPath).Output()
	if err != nil {
		// Fallback: create rootfs tar.gz alongside the image
		fmt.Fprintf(w, "  (losetup not available — creating rootfs.tar.gz fallback)\n")
		tarPath := imgPath + ".tar.gz"
		cmd := exec.Command("tar", "czf", tarPath, "-C", rootfs, ".")
		return cmd.Run()
	}
	loopDev := strings.TrimSpace(string(out))
	defer exec.Command("losetup", "-d", loopDev).Run()

	for i, p := range recipe.Partitions {
		partDev := fmt.Sprintf("%sp%d", loopDev, i+1)
		fmt.Fprintf(w, "  Formatting %s (%s)...\n", p.Label, p.Type)

		switch p.Type {
		case "vfat":
			exec.Command("mkfs.vfat", "-n", strings.ToUpper(p.Label), partDev).Run()
			mountDir := filepath.Join(filepath.Dir(imgPath), "mnt-boot")
			os.MkdirAll(mountDir, 0755)
			if err := exec.Command("mount", partDev, mountDir).Run(); err != nil {
				continue
			}
			for _, pattern := range p.Contents {
				matches, _ := filepath.Glob(filepath.Join(rootfs, "boot", pattern))
				for _, f := range matches {
					exec.Command("cp", f, mountDir).Run()
				}
			}
			exec.Command("umount", mountDir).Run()

		case "ext4":
			exec.Command("mkfs.ext4", "-L", p.Label, "-q", partDev).Run()
			if p.Root {
				mountDir := filepath.Join(filepath.Dir(imgPath), "mnt-rootfs")
				os.MkdirAll(mountDir, 0755)
				if err := exec.Command("mount", partDev, mountDir).Run(); err != nil {
					continue
				}
				exec.Command("cp", "-a", rootfs+"/.", mountDir+"/").Run()
				exec.Command("umount", mountDir).Run()
			}
		}
	}

	return nil
}

func parseSizeMB(size string) int {
	if size == "fill" || size == "" {
		return 0
	}
	var n int
	if _, err := fmt.Sscanf(size, "%dM", &n); err == nil {
		return n
	}
	if _, err := fmt.Sscanf(size, "%dG", &n); err == nil {
		return n * 1024
	}
	return 0
}
