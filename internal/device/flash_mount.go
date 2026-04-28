package device

import (
	"bufio"
	"os"
	"strings"
)

// MountedPartition describes a mounted partition of a target disk.
type MountedPartition struct {
	Source     string // /dev/sdb1
	Mountpoint string // /media/cbrake/BOOT
}

// MountedPartitionsFor returns mounted partitions whose source device is
// the given whole-disk device or a partition of it. devicePath is expected
// to be a whole-disk path like /dev/sdb (use parentDisk to normalize).
func MountedPartitionsFor(devicePath string) ([]MountedPartition, error) {
	return mountedPartitionsFor(devicePath, "/proc/self/mountinfo")
}

func mountedPartitionsFor(devicePath, mountInfoPath string) ([]MountedPartition, error) {
	f, err := os.Open(mountInfoPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []MountedPartition
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// mountinfo format:
		//   36 35 98:0 /mnt1 /mnt/parent rw,noatime - ext3 /dev/sdb1 rw,errors=continue
		// We need fields after the " - " separator: fs_type, source, options.
		dash := strings.Index(line, " - ")
		if dash < 0 {
			continue
		}
		head := strings.Fields(line[:dash])
		tail := strings.Fields(line[dash+3:])
		if len(head) < 5 || len(tail) < 2 {
			continue
		}
		mountpoint := head[4]
		source := tail[1]

		if !sourceMatchesDisk(source, devicePath) {
			continue
		}
		key := source + "\x00" + mountpoint
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, MountedPartition{Source: source, Mountpoint: mountpoint})
	}
	return out, scanner.Err()
}

// sourceMatchesDisk returns true when the mountinfo source path is the
// disk itself or a partition of it. Handles both /dev/sdb / /dev/sdb1
// and /dev/mmcblk0 / /dev/mmcblk0p1 naming.
func sourceMatchesDisk(source, devicePath string) bool {
	if !strings.HasPrefix(source, "/dev/") {
		return false
	}
	if source == devicePath {
		return true
	}
	if !strings.HasPrefix(source, devicePath) {
		return false
	}
	suffix := source[len(devicePath):]
	if suffix == "" {
		return true
	}
	// Partition suffix is either digits (sdb1) or pN+digits (mmcblk0p1).
	if suffix[0] == 'p' {
		suffix = suffix[1:]
	}
	if suffix == "" {
		return false
	}
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
