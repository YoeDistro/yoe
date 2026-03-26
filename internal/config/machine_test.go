package config

import (
	"path/filepath"
	"testing"
)

func TestParseMachineConfig(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-project", "machines", "beaglebone-black.toml")
	machine, err := ParseMachineConfig(path)
	if err != nil {
		t.Fatalf("ParseMachineConfig(%q): %v", path, err)
	}

	if machine.Machine.Name != "beaglebone-black" {
		t.Errorf("Name = %q, want %q", machine.Machine.Name, "beaglebone-black")
	}
	if machine.Machine.Arch != "arm64" {
		t.Errorf("Arch = %q, want %q", machine.Machine.Arch, "arm64")
	}
	if machine.Kernel.Repo != "https://github.com/beagleboard/linux.git" {
		t.Errorf("Kernel.Repo = %q, want correct URL", machine.Kernel.Repo)
	}
	if machine.Kernel.Defconfig != "bb.org_defconfig" {
		t.Errorf("Kernel.Defconfig = %q, want %q", machine.Kernel.Defconfig, "bb.org_defconfig")
	}
	if len(machine.Kernel.DeviceTrees) != 1 || machine.Kernel.DeviceTrees[0] != "am335x-boneblack.dtb" {
		t.Errorf("Kernel.DeviceTrees = %v, want [am335x-boneblack.dtb]", machine.Kernel.DeviceTrees)
	}
	if machine.Bootloader.Type != "u-boot" {
		t.Errorf("Bootloader.Type = %q, want %q", machine.Bootloader.Type, "u-boot")
	}
	if machine.Image.PartitionLayout != "partitions/bbb.toml" {
		t.Errorf("Image.PartitionLayout = %q, want %q", machine.Image.PartitionLayout, "partitions/bbb.toml")
	}
}

func TestParseMachineConfig_QEMU(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-project", "machines", "qemu-x86_64.toml")
	machine, err := ParseMachineConfig(path)
	if err != nil {
		t.Fatalf("ParseMachineConfig(%q): %v", path, err)
	}

	if machine.Machine.QEMU == nil {
		t.Fatal("expected QEMU config, got nil")
	}
	if machine.Machine.QEMU.Machine != "q35" {
		t.Errorf("QEMU.Machine = %q, want %q", machine.Machine.QEMU.Machine, "q35")
	}
	if machine.Machine.QEMU.Memory != "1G" {
		t.Errorf("QEMU.Memory = %q, want %q", machine.Machine.QEMU.Memory, "1G")
	}
}

func TestParseMachineConfig_InvalidArch(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "invalid-project", "bad-arch-machine.toml")
	_, err := ParseMachineConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid arch, got nil")
	}
}
