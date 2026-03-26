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
// Uses raw file operations — no loop devices or mounting needed (works inside
// Docker without --privileged).
func GenerateDiskImage(rootfs, imgPath string, recipe *yoestar.Recipe, w io.Writer) error {
	partitions := recipe.Partitions
	if len(partitions) == 0 {
		partitions = []yoestar.Partition{
			{Label: "rootfs", Type: "ext4", Size: "512M", Root: true},
		}
	}

	// Calculate sizes
	totalMB := 0
	for _, p := range partitions {
		size := parseSizeMB(p.Size)
		if size == 0 {
			size = 512
		}
		totalMB += size
	}

	fmt.Fprintf(w, "  Creating %dMB disk image...\n", totalMB)

	// Create sparse image
	if err := createSparseImage(imgPath, totalMB); err != nil {
		return fmt.Errorf("creating image: %w", err)
	}

	// Partition with sfdisk
	if err := partitionImage(imgPath, partitions, w); err != nil {
		return fmt.Errorf("partitioning: %w", err)
	}

	// Create individual partition images and dd them into the disk image
	offsetMB := 1 // 1MB for GPT header
	for _, p := range partitions {
		sizeMB := parseSizeMB(p.Size)
		if sizeMB == 0 {
			sizeMB = totalMB - offsetMB
		}

		fmt.Fprintf(w, "  Creating %s partition (%s, %dMB)...\n", p.Label, p.Type, sizeMB)

		partImg := imgPath + "." + p.Label + ".part"
		defer os.Remove(partImg)

		switch p.Type {
		case "vfat":
			if err := createVfatPartition(partImg, sizeMB, rootfs, p, w); err != nil {
				return fmt.Errorf("vfat %s: %w", p.Label, err)
			}
		case "ext4":
			if err := createExt4Partition(partImg, sizeMB, rootfs, p, w); err != nil {
				return fmt.Errorf("ext4 %s: %w", p.Label, err)
			}
		}

		// DD the partition image into the disk image at the right offset
		if _, err := os.Stat(partImg); err == nil {
			cmd := exec.Command("dd",
				"if="+partImg,
				"of="+imgPath,
				"bs=1M",
				fmt.Sprintf("seek=%d", offsetMB),
				"conv=notrunc",
			)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("dd partition %s: %w", p.Label, err)
			}
		}

		offsetMB += sizeMB
	}

	info, _ := os.Stat(imgPath)
	if info != nil {
		fmt.Fprintf(w, "  Disk image: %s (%dMB)\n", imgPath, info.Size()/(1024*1024))
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
	if _, err := exec.LookPath("sfdisk"); err != nil {
		fmt.Fprintln(w, "  (sfdisk not available — skipping partitioning)")
		return nil
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

	fmt.Fprintln(w, "  Partitioning (GPT)...")
	cmd := exec.Command("sfdisk", "--quiet", imgPath)
	cmd.Stdin = strings.NewReader(script)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// createVfatPartition creates a FAT32 filesystem image and copies boot files.
// Uses mkfs.vfat + mcopy (mtools) — no loop device or mounting needed.
func createVfatPartition(partImg string, sizeMB int, rootfs string, p yoestar.Partition, w io.Writer) error {
	// Create the partition image file
	if err := createSparseImage(partImg, sizeMB); err != nil {
		return err
	}

	// Format as FAT32
	cmd := exec.Command("mkfs.vfat", "-n", strings.ToUpper(p.Label), partImg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mkfs.vfat: %s\n%s", err, out)
	}

	// Copy boot files using mcopy if available, otherwise dd raw
	if _, err := exec.LookPath("mcopy"); err == nil {
		for _, pattern := range p.Contents {
			matches, _ := filepath.Glob(filepath.Join(rootfs, "boot", pattern))
			for _, f := range matches {
				cmd := exec.Command("mcopy", "-i", partImg, f, "::/"+filepath.Base(f))
				if out, err := cmd.CombinedOutput(); err != nil {
					fmt.Fprintf(w, "    mcopy %s: %s\n", filepath.Base(f), string(out))
				} else {
					fmt.Fprintf(w, "    boot: %s\n", filepath.Base(f))
				}
			}
		}
	} else {
		fmt.Fprintln(w, "    (mcopy not available — boot partition empty)")
	}

	return nil
}

// createExt4Partition creates an ext4 filesystem image populated from rootfs.
// Uses mkfs.ext4 -d (populate from directory) — no loop device or mounting.
func createExt4Partition(partImg string, sizeMB int, rootfs string, p yoestar.Partition, w io.Writer) error {
	if !p.Root {
		// Non-root ext4 partition — just create empty
		if err := createSparseImage(partImg, sizeMB); err != nil {
			return err
		}
		cmd := exec.Command("mkfs.ext4", "-q", "-L", p.Label, partImg)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("mkfs.ext4: %s\n%s", err, out)
		}
		return nil
	}

	// Root partition — create and populate from rootfs using mkfs.ext4 -d
	if err := createSparseImage(partImg, sizeMB); err != nil {
		return err
	}

	cmd := exec.Command("mkfs.ext4", "-q", "-L", p.Label, "-d", rootfs, partImg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mkfs.ext4 -d: %s\n%s", err, out)
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
