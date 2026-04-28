package device

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// fakeBlock builds a minimal /sys/class/block fixture for one device.
// device fields populate the corresponding sysfs files; bus, vendor, model
// can be empty to skip writing those.
type fakeBlock struct {
	name      string
	removable string
	size      string
	ro        string
	bus       string // "usb", "mmc", "scsi", "nvme", or "" to skip
	vendor    string
	model     string
	partition bool // create the "partition" file (mark as partition)
}

func writeFakeSys(t *testing.T, sysroot string, blocks []fakeBlock) {
	t.Helper()
	for _, b := range blocks {
		blockDir := filepath.Join(sysroot, "class", "block", b.name)
		if err := os.MkdirAll(blockDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(blockDir, "removable"), b.removable+"\n")
		writeFile(t, filepath.Join(blockDir, "size"), b.size+"\n")
		writeFile(t, filepath.Join(blockDir, "ro"), b.ro+"\n")
		if b.partition {
			writeFile(t, filepath.Join(blockDir, "partition"), "1\n")
		}
		if b.bus != "" {
			// /sys/class/block/<n>/device → ../../bus/<bus>/devices/<n>
			devDir := filepath.Join(sysroot, "bus", b.bus, "devices", b.name)
			if err := os.MkdirAll(devDir, 0o755); err != nil {
				t.Fatal(err)
			}
			busSubsystem := filepath.Join(sysroot, "bus", b.bus)
			// Make subsystem a real dir + a "subsystem" symlink in devDir.
			if err := os.Symlink(busSubsystem, filepath.Join(devDir, "subsystem")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(devDir, filepath.Join(blockDir, "device")); err != nil {
				t.Fatal(err)
			}
			if b.vendor != "" {
				writeFile(t, filepath.Join(devDir, "vendor"), b.vendor+"\n")
			}
			if b.model != "" {
				writeFile(t, filepath.Join(devDir, "model"), b.model+"\n")
			}
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListCandidatesFiltersBlocks(t *testing.T) {
	sysroot := t.TempDir()
	writeFakeSys(t, sysroot, []fakeBlock{
		// Removable USB stick — keep
		{name: "sdb", removable: "1", size: "62333952", ro: "0", bus: "usb", vendor: "Generic", model: "USB Flash Disk"},
		// Internal SATA disk — drop (not removable, scsi/ata not in keep set)
		{name: "sda", removable: "0", size: "500000000", ro: "0", bus: "scsi", vendor: "Samsung", model: "SSD 970"},
		// Internal NVMe — drop
		{name: "nvme0n1", removable: "0", size: "1000000000", ro: "0", bus: "nvme", vendor: "WD", model: "Blue"},
		// MMC card — keep (bus=mmc passes even if removable=0)
		{name: "mmcblk0", removable: "0", size: "121634816", ro: "0", bus: "mmc", model: "SD64G"},
		// Loopback — drop by name
		{name: "loop0", removable: "0", size: "0", ro: "0"},
		// Optical — drop by name
		{name: "sr0", removable: "1", size: "0", ro: "1"},
		// Ramdisk — drop by name
		{name: "ram0", removable: "0", size: "8192", ro: "0"},
		// Partition — drop (partition file present)
		{name: "sdb1", removable: "1", size: "62333952", ro: "0", bus: "usb", partition: true},
		// Removable but no media — drop (size == 0)
		{name: "sdc", removable: "1", size: "0", ro: "0", bus: "usb"},
		// Read-only USB — drop
		{name: "sdd", removable: "1", size: "1024", ro: "1", bus: "usb"},
	})

	got, err := listCandidates(sysroot, nil)
	if err != nil {
		t.Fatal(err)
	}

	wantPaths := []string{"/dev/mmcblk0", "/dev/sdb"}
	var gotPaths []string
	for _, c := range got {
		gotPaths = append(gotPaths, c.Path)
	}
	sort.Strings(gotPaths)
	if !equalSlice(gotPaths, wantPaths) {
		t.Errorf("got %v, want %v", gotPaths, wantPaths)
	}

	// Verify field population for the USB candidate.
	var sdb *Candidate
	for i := range got {
		if got[i].Path == "/dev/sdb" {
			sdb = &got[i]
		}
	}
	if sdb == nil {
		t.Fatal("expected /dev/sdb in result")
	}
	if sdb.Bus != "usb" {
		t.Errorf("sdb bus = %q, want usb", sdb.Bus)
	}
	if sdb.Vendor != "Generic" || sdb.Model != "USB Flash Disk" {
		t.Errorf("sdb vendor/model = %q/%q, want Generic / USB Flash Disk", sdb.Vendor, sdb.Model)
	}
	wantBytes := int64(62333952) * 512
	if sdb.Size != wantBytes {
		t.Errorf("sdb size = %d, want %d", sdb.Size, wantBytes)
	}
}

func TestListCandidatesExcludesSystemDisks(t *testing.T) {
	sysroot := t.TempDir()
	writeFakeSys(t, sysroot, []fakeBlock{
		// Removable USB stick
		{name: "sdb", removable: "1", size: "62333952", ro: "0", bus: "usb"},
		// MMC eMMC that hosts the running system
		{name: "mmcblk0", removable: "0", size: "121634816", ro: "0", bus: "mmc"},
	})

	systemBlocked := map[string]bool{"/dev/mmcblk0": true}
	got, err := listCandidates(sysroot, systemBlocked)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/dev/sdb" {
		t.Errorf("expected only /dev/sdb, got %+v", got)
	}
}

func TestFormatSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1000, "1.0 KB"},
		{1500, "1.5 KB"},
		{1_000_000, "1.0 MB"},
		{31_900_000_000, "31.9 GB"},
	}
	for _, c := range cases {
		if got := FormatSize(c.in); got != c.want {
			t.Errorf("FormatSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
