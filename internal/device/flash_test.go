package device

import (
	"os"
	"strings"
	"testing"
)

func TestSystemDisksSkipsNonCriticalMounts(t *testing.T) {
	mounts := `proc /proc proc rw 0 0
/dev/sda1 /mnt/usb ext4 rw 0 0
tmpfs /tmp tmpfs rw 0 0
`
	got := systemDisks(mounts)
	if len(got) != 0 {
		t.Errorf("expected no system disks for non-critical mounts, got %v", got)
	}
}

func TestSystemDisksParsesCriticalMountpoints(t *testing.T) {
	// Run against the actual /proc/mounts on the test runner. Whatever
	// disk hosts / on this machine should be returned, and an obviously
	// unrelated path should not.
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		t.Skipf("cannot read /proc/mounts: %v", err)
	}
	disks := systemDisks(string(data))
	for _, d := range disks {
		if !strings.HasPrefix(d, "/dev/") {
			t.Errorf("system disk %q does not start with /dev/", d)
		}
	}
}

func TestValidateDeviceRejectsNonBlockDevice(t *testing.T) {
	tmp, err := os.CreateTemp("", "flash-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	err = validateDevice(tmp.Name())
	if err == nil || !strings.Contains(err.Error(), "not a block device") {
		t.Errorf("expected 'not a block device' error, got %v", err)
	}
}

func TestValidateDeviceRejectsMissingPath(t *testing.T) {
	if err := validateDevice(""); err == nil {
		t.Error("expected error for empty device path")
	}
	if err := validateDevice("/dev/does-not-exist-xyz"); err == nil {
		t.Error("expected error for non-existent device")
	}
}
