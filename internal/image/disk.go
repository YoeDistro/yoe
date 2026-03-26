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

	// Install syslinux bootloader (MBR + VBR + ldlinux.sys)
	if err := installBootloader(imgPath, rootfs, recipe, w); err != nil {
		fmt.Fprintf(w, "  Warning: could not install bootloader: %v\n", err)
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

	script := "label: dos\n"
	for i, p := range partitions {
		size := ""
		sizeMB := parseSizeMB(p.Size)
		if sizeMB > 0 && i < len(partitions)-1 {
			// Specify size only for non-last partitions; last gets remaining space
			size = fmt.Sprintf("size=%dMiB, ", sizeMB)
		}
		ptype := "83" // Linux
		if p.Type == "vfat" {
			ptype = "c" // W95 FAT32 (LBA)
		}
		bootable := ""
		if p.Root {
			bootable = ", bootable"
		}
		script += fmt.Sprintf("%stype=%s%s\n", size, ptype, bootable)
	}

	fmt.Fprintln(w, "  Partitioning (MBR)...")
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

// installBootloader writes syslinux MBR code and runs extlinux --install
// on the root partition to set up the VBR and ldlinux.sys.
func installBootloader(imgPath, rootfs string, recipe *yoestar.Recipe, w io.Writer) error {
	// Write MBR boot code
	// Try rootfs first (from syslinux recipe), then container's syslinux
	mbrBin := filepath.Join(rootfs, "usr", "share", "syslinux", "mbr.bin")
	if _, err := os.Stat(mbrBin); os.IsNotExist(err) {
		mbrBin = "/usr/share/syslinux/mbr.bin"
		if _, err := os.Stat(mbrBin); os.IsNotExist(err) {
			return fmt.Errorf("syslinux mbr.bin not found")
		}
	}

	mbrData, err := os.ReadFile(mbrBin)
	if err != nil {
		return err
	}
	if len(mbrData) > 440 {
		mbrData = mbrData[:440]
	}

	img, err := os.OpenFile(imgPath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := img.WriteAt(mbrData, 0); err != nil {
		img.Close()
		return fmt.Errorf("writing MBR: %w", err)
	}
	img.Close()
	fmt.Fprintln(w, "  Installed syslinux MBR boot code")

	// Run extlinux --install on the root partition using a loop device
	// This writes the VBR and ldlinux.sys to the right disk sectors
	if _, err := exec.LookPath("losetup"); err != nil {
		fmt.Fprintln(w, "  (losetup not available — skipping extlinux install)")
		return nil
	}

	// Find the root partition offset
	offsetBytes := int64(1024 * 1024) // 1MB default (after MBR)
	for _, p := range recipe.Partitions {
		if p.Root {
			break
		}
		sizeMB := parseSizeMB(p.Size)
		if sizeMB > 0 {
			offsetBytes += int64(sizeMB) * 1024 * 1024
		}
	}

	// Set up loop device for the partition
	out, err := exec.Command("losetup", "--find", "--show",
		"--offset", fmt.Sprintf("%d", offsetBytes),
		"--sizelimit", fmt.Sprintf("%d", 512*1024*1024),
		imgPath).CombinedOutput()
	if err != nil {
		fmt.Fprintf(w, "  (losetup failed: %v: %s — skipping extlinux install)\n", err, strings.TrimSpace(string(out)))
		return nil
	}
	loopDev := strings.TrimSpace(string(out))
	defer exec.Command("losetup", "-d", loopDev).Run()

	// Mount the partition
	mountDir, err := os.MkdirTemp("", "yoe-extlinux-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(mountDir)

	if err := exec.Command("mount", loopDev, mountDir).Run(); err != nil {
		fmt.Fprintf(w, "  (mount failed: %v — skipping extlinux install)\n", err)
		return nil
	}
	defer exec.Command("umount", mountDir).Run()

	// Run extlinux --install
	extlinuxDir := filepath.Join(mountDir, "boot", "extlinux")
	cmd := exec.Command("extlinux", "--install", extlinuxDir)
	if cmdOut, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(w, "  extlinux --install failed: %s\n%s", err, cmdOut)
		return nil // non-fatal — image still has the files, just no VBR
	}

	fmt.Fprintln(w, "  Installed extlinux bootloader")
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
