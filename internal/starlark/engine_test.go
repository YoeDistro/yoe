package starlark

import (
	"testing"
)

func TestEvalProject(t *testing.T) {
	src := `
project(
    name = "test-project",
    version = "0.1.0",
    defaults = defaults(machine = "qemu-arm64", image = "base-image"),
    repository = repository(path = "/var/cache/yoe-ng/repo"),
    cache = cache(path = "/var/cache/yoe-ng/build"),
)
`
	eng := NewEngine()
	if err := eng.ExecString("PROJECT.star", src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	proj := eng.Project()
	if proj == nil {
		t.Fatal("Project() returned nil")
	}
	if proj.Name != "test-project" {
		t.Errorf("Name = %q, want %q", proj.Name, "test-project")
	}
	if proj.Defaults.Machine != "qemu-arm64" {
		t.Errorf("Defaults.Machine = %q, want %q", proj.Defaults.Machine, "qemu-arm64")
	}
	if proj.Defaults.Image != "base-image" {
		t.Errorf("Defaults.Image = %q, want %q", proj.Defaults.Image, "base-image")
	}
	if proj.Repository.Path != "/var/cache/yoe-ng/repo" {
		t.Errorf("Repository.Path = %q, want %q", proj.Repository.Path, "/var/cache/yoe-ng/repo")
	}
	if proj.Cache.Path != "/var/cache/yoe-ng/build" {
		t.Errorf("Cache.Path = %q, want %q", proj.Cache.Path, "/var/cache/yoe-ng/build")
	}
}

func TestEvalMachine(t *testing.T) {
	src := `
machine(
    name = "beaglebone-black",
    arch = "arm64",
    description = "BeagleBone Black",
    kernel = kernel(
        repo = "https://github.com/beagleboard/linux.git",
        branch = "6.6",
        defconfig = "bb.org_defconfig",
        device_trees = ["am335x-boneblack.dtb"],
    ),
    uboot = uboot(
        repo = "https://github.com/beagleboard/u-boot.git",
        branch = "v2024.01",
        defconfig = "am335x_evm_defconfig",
    ),
)
`
	eng := NewEngine()
	if err := eng.ExecString("machines/bbb.star", src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	machines := eng.Machines()
	m, ok := machines["beaglebone-black"]
	if !ok {
		t.Fatal("machine 'beaglebone-black' not found")
	}
	if m.Arch != "arm64" {
		t.Errorf("Arch = %q, want %q", m.Arch, "arm64")
	}
	if m.Kernel.Defconfig != "bb.org_defconfig" {
		t.Errorf("Kernel.Defconfig = %q, want %q", m.Kernel.Defconfig, "bb.org_defconfig")
	}
	if len(m.Kernel.DeviceTrees) != 1 {
		t.Errorf("Kernel.DeviceTrees = %v, want 1 entry", m.Kernel.DeviceTrees)
	}
	if m.Bootloader.Type != "u-boot" {
		t.Errorf("Bootloader.Type = %q, want %q", m.Bootloader.Type, "u-boot")
	}
	if m.Bootloader.Defconfig != "am335x_evm_defconfig" {
		t.Errorf("Bootloader.Defconfig = %q, want %q", m.Bootloader.Defconfig, "am335x_evm_defconfig")
	}
}

func TestEvalMachineQEMU(t *testing.T) {
	src := `
machine(
    name = "qemu-x86_64",
    arch = "x86_64",
    kernel = kernel(recipe = "linux-qemu", cmdline = "console=ttyS0"),
    qemu = qemu_config(machine = "q35", cpu = "host", memory = "1G", firmware = "ovmf"),
)
`
	eng := NewEngine()
	if err := eng.ExecString("machines/qemu.star", src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	m := eng.Machines()["qemu-x86_64"]
	if m.QEMU == nil {
		t.Fatal("expected QEMU config, got nil")
	}
	if m.QEMU.Machine != "q35" {
		t.Errorf("QEMU.Machine = %q, want %q", m.QEMU.Machine, "q35")
	}
	if m.QEMU.Memory != "1G" {
		t.Errorf("QEMU.Memory = %q, want %q", m.QEMU.Memory, "1G")
	}
}

func TestEvalPackageRecipe(t *testing.T) {
	src := `
package(
    name = "openssh",
    version = "9.6p1",
    source = "https://cdn.openbsd.org/pub/OpenBSD/OpenSSH/portable/openssh-9.6p1.tar.gz",
    sha256 = "abc123",
    deps = ["zlib", "openssl"],
    runtime_deps = ["zlib", "openssl"],
    build = [
        "./configure --prefix=$PREFIX",
        "make -j$NPROC",
        "make DESTDIR=$DESTDIR install",
    ],
    services = ["sshd"],
    conffiles = ["/etc/ssh/sshd_config"],
)
`
	eng := NewEngine()
	if err := eng.ExecString("recipes/openssh.star", src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	recipes := eng.Recipes()
	r, ok := recipes["openssh"]
	if !ok {
		t.Fatal("recipe 'openssh' not found")
	}
	if r.Class != "package" {
		t.Errorf("Class = %q, want %q", r.Class, "package")
	}
	if r.Version != "9.6p1" {
		t.Errorf("Version = %q, want %q", r.Version, "9.6p1")
	}
	if len(r.Deps) != 2 {
		t.Errorf("Deps = %v, want 2 entries", r.Deps)
	}
	if len(r.Build) != 3 {
		t.Errorf("Build = %v, want 3 entries", r.Build)
	}
	if len(r.Services) != 1 || r.Services[0] != "sshd" {
		t.Errorf("Services = %v, want [sshd]", r.Services)
	}
}

func TestEvalAutotoolsRecipe(t *testing.T) {
	src := `
autotools(
    name = "zlib",
    version = "1.3.1",
    source = "https://zlib.net/zlib-1.3.1.tar.gz",
    configure_args = ["--prefix=$PREFIX"],
)
`
	eng := NewEngine()
	if err := eng.ExecString("recipes/zlib.star", src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	r := eng.Recipes()["zlib"]
	if r.Class != "autotools" {
		t.Errorf("Class = %q, want %q", r.Class, "autotools")
	}
}

func TestEvalGoBinaryRecipe(t *testing.T) {
	src := `
go_binary(
    name = "myapp",
    version = "1.2.3",
    source = "https://github.com/example/myapp.git",
    tag = "v1.2.3",
    package = "./cmd/myapp",
)
`
	eng := NewEngine()
	if err := eng.ExecString("recipes/myapp.star", src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	r := eng.Recipes()["myapp"]
	if r.Class != "go" {
		t.Errorf("Class = %q, want %q", r.Class, "go")
	}
	if r.GoPackage != "./cmd/myapp" {
		t.Errorf("GoPackage = %q, want %q", r.GoPackage, "./cmd/myapp")
	}
}

func TestEvalImageRecipe(t *testing.T) {
	src := `
image(
    name = "base-image",
    version = "1.0.0",
    packages = ["openssh", "myapp"],
    hostname = "yoe",
    services = ["sshd"],
    partitions = [
        partition(label="boot", type="vfat", size="64M"),
        partition(label="rootfs", type="ext4", size="fill", root=True),
    ],
)
`
	eng := NewEngine()
	if err := eng.ExecString("recipes/base-image.star", src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	recipes := eng.Recipes()
	r, ok := recipes["base-image"]
	if !ok {
		t.Fatal("recipe 'base-image' not found")
	}
	if r.Class != "image" {
		t.Errorf("Class = %q, want %q", r.Class, "image")
	}
	if len(r.Packages) != 2 {
		t.Errorf("Packages = %v, want 2 entries", r.Packages)
	}
	if r.Hostname != "yoe" {
		t.Errorf("Hostname = %q, want %q", r.Hostname, "yoe")
	}
	if len(r.Partitions) != 2 {
		t.Errorf("Partitions = %v, want 2 entries", r.Partitions)
	}
	if !r.Partitions[1].Root {
		t.Error("Partitions[1].Root = false, want true")
	}
	if r.Partitions[0].Size != "64M" {
		t.Errorf("Partitions[0].Size = %q, want %q", r.Partitions[0].Size, "64M")
	}
}

func TestEvalInvalidArch(t *testing.T) {
	src := `machine(name = "bad", arch = "mips")`
	eng := NewEngine()
	err := eng.ExecString("machines/bad.star", src)
	if err == nil {
		t.Fatal("expected error for invalid arch, got nil")
	}
}

func TestEvalRecipeWithPatches(t *testing.T) {
	src := `
package(
    name = "busybox",
    version = "1.36.1",
    source = "https://busybox.net/downloads/busybox-1.36.1.tar.bz2",
    patches = [
        "patches/busybox/fix-ash-segfault.patch",
        "patches/busybox/add-custom-applet.patch",
    ],
    build = ["make -j$NPROC", "make DESTDIR=$DESTDIR install"],
)
`
	eng := NewEngine()
	if err := eng.ExecString("recipes/busybox.star", src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	r := eng.Recipes()["busybox"]
	if len(r.Patches) != 2 {
		t.Errorf("Patches = %v, want 2 entries", r.Patches)
	}
	if r.Patches[0] != "patches/busybox/fix-ash-segfault.patch" {
		t.Errorf("Patches[0] = %q, want fix-ash-segfault.patch", r.Patches[0])
	}
}

func TestEvalPackageRequiresBuild(t *testing.T) {
	src := `package(name = "broken", version = "1.0.0")`
	eng := NewEngine()
	err := eng.ExecString("recipes/broken.star", src)
	if err == nil {
		t.Fatal("expected error for package with no build steps, got nil")
	}
}

func TestEvalProjectDuplicate(t *testing.T) {
	src := `
project(name = "first", version = "1.0.0")
project(name = "second", version = "2.0.0")
`
	eng := NewEngine()
	err := eng.ExecString("PROJECT.star", src)
	if err == nil {
		t.Fatal("expected error for duplicate project(), got nil")
	}
}
