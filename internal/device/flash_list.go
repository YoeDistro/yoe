package device

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Candidate describes a removable block device suitable for flashing.
type Candidate struct {
	Path     string // /dev/sdb, /dev/mmcblk0, ...
	Size     int64  // bytes
	Bus      string // usb, mmc, scsi, ata, ""
	Vendor   string
	Model    string
	ReadOnly bool
}

// ListCandidates returns block devices that pass the removable / bus
// heuristic from balena's etcher-sdk: removable=1 OR bus in {usb, mmc},
// non-zero size, not read-only — minus any device that hosts a critical
// system mountpoint (/, /boot, /boot/efi, /usr).
func ListCandidates() ([]Candidate, error) {
	systemBlocked := map[string]bool{}
	if data, err := os.ReadFile("/proc/mounts"); err == nil {
		for _, d := range systemDisks(string(data)) {
			systemBlocked[d] = true
		}
	}
	return listCandidates("/sys", systemBlocked)
}

func listCandidates(sysroot string, systemBlocked map[string]bool) ([]Candidate, error) {
	entries, err := os.ReadDir(filepath.Join(sysroot, "class", "block"))
	if err != nil {
		return nil, fmt.Errorf("read /sys/class/block: %w", err)
	}

	var out []Candidate
	for _, e := range entries {
		name := e.Name()
		if skipByName(name) {
			continue
		}
		blockDir := filepath.Join(sysroot, "class", "block", name)

		// Skip partitions: /sys/class/block/<name>/partition exists for them.
		if _, err := os.Stat(filepath.Join(blockDir, "partition")); err == nil {
			continue
		}

		removable := readUint(filepath.Join(blockDir, "removable")) == 1
		ro := readUint(filepath.Join(blockDir, "ro")) == 1
		sectors := readInt64(filepath.Join(blockDir, "size"))
		bus := readBus(blockDir)

		if !(removable || bus == "usb" || bus == "mmc") {
			continue
		}
		if sectors == 0 {
			continue
		}
		if ro {
			continue
		}
		if systemBlocked["/dev/"+name] {
			continue
		}

		out = append(out, Candidate{
			Path:     "/dev/" + name,
			Size:     sectors * 512,
			Bus:      bus,
			Vendor:   strings.TrimSpace(readString(filepath.Join(blockDir, "device", "vendor"))),
			Model:    strings.TrimSpace(readString(filepath.Join(blockDir, "device", "model"))),
			ReadOnly: ro,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// skipByName drops devices whose names indicate they are not viable
// flash targets (loopback, optical, ramdisks, dm/md mappers).
func skipByName(name string) bool {
	for _, prefix := range []string{"loop", "sr", "ram", "dm-", "md", "zram", "fd"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// readBus walks the device subsystem chain to determine the bus type
// (usb, mmc, scsi, ata, nvme). Returns "" if it can't be determined.
func readBus(blockDir string) string {
	// /sys/class/block/<name>/device is a symlink into the bus tree.
	dev, err := filepath.EvalSymlinks(filepath.Join(blockDir, "device"))
	if err != nil {
		return ""
	}
	// Walk up looking for a parent whose subsystem link points to a
	// recognizable bus.
	for cur := dev; cur != "/" && cur != "."; cur = filepath.Dir(cur) {
		sub, err := os.Readlink(filepath.Join(cur, "subsystem"))
		if err != nil {
			continue
		}
		bus := filepath.Base(sub)
		switch bus {
		case "usb", "mmc", "scsi", "ata", "nvme", "sdio":
			return bus
		}
	}
	return ""
}

func readString(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func readUint(path string) uint64 {
	s := strings.TrimSpace(readString(path))
	if s == "" {
		return 0
	}
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}

func readInt64(path string) int64 {
	s := strings.TrimSpace(readString(path))
	if s == "" {
		return 0
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// FormatSize renders a byte count as a human-readable size (e.g. "31.9 GB").
func FormatSize(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
