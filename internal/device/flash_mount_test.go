package device

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceMatchesDisk(t *testing.T) {
	cases := []struct {
		source, disk string
		want         bool
	}{
		{"/dev/sdb1", "/dev/sdb", true},
		{"/dev/sdb", "/dev/sdb", true},
		{"/dev/sdb12", "/dev/sdb", true},
		{"/dev/mmcblk0p1", "/dev/mmcblk0", true},
		{"/dev/mmcblk0", "/dev/mmcblk0", true},
		{"/dev/sda1", "/dev/sdb", false},
		{"/dev/sdba", "/dev/sdb", false}, // sdba is a different disk
		{"tmpfs", "/dev/sdb", false},
	}
	for _, c := range cases {
		if got := sourceMatchesDisk(c.source, c.disk); got != c.want {
			t.Errorf("sourceMatchesDisk(%q,%q) = %v, want %v", c.source, c.disk, got, c.want)
		}
	}
}

func TestMountedPartitionsForParsesMountinfo(t *testing.T) {
	mountinfo := `22 28 0:21 / /sys rw,relatime shared:7 - sysfs sysfs rw
36 28 8:17 / /media/cbrake/BOOT rw,relatime shared:60 - vfat /dev/sdb1 rw,fmask=0022
37 28 8:18 / /media/cbrake/ROOT rw,relatime shared:61 - ext4 /dev/sdb2 rw
40 28 7:0 / /snap/foo rw,relatime shared:80 - squashfs /dev/loop0 ro
50 28 8:0 / /mnt/other rw,relatime shared:90 - ext4 /dev/sdc1 rw
`
	tmp := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(tmp, []byte(mountinfo), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := mountedPartitionsFor("/dev/sdb", tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d mounts, want 2: %+v", len(got), got)
	}
	want := map[string]string{
		"/dev/sdb1": "/media/cbrake/BOOT",
		"/dev/sdb2": "/media/cbrake/ROOT",
	}
	for _, m := range got {
		if want[m.Source] != m.Mountpoint {
			t.Errorf("unexpected mount %+v", m)
		}
	}
}

func TestMountedPartitionsForEmpty(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(tmp, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := mountedPartitionsFor("/dev/sdb", tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}
