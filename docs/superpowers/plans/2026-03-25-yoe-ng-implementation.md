# Yoe-NG Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `yoe`, a Go CLI tool that builds packages from TOML recipes,
assembles root filesystem images, and manages an embedded Linux distribution — a
simpler alternative to Yocto.

**Architecture:** Single Go binary with stdlib CLI (no framework — switch/case
dispatch like brun), BurntSushi/toml for metadata parsing, bubblewrap for build
isolation, and apk-tools for package management. Two-phase resolve-then-build
model inspired by Google GN. Content-addressed caching at every layer.

**Tech Stack:** Go 1.22+, Go stdlib (CLI — no Cobra), BurntSushi/toml (TOML
parsing), Bubble Tea (TUI), bubblewrap (sandboxing), apk-tools (package
management), systemd-repart (disk images)

---

## Phase Overview

This project is broken into 9 phases. Each phase produces working, testable
software. Phases 1-3 are pure Go with no external system dependencies (testable
on any dev machine). Phases 4+ require Linux with bubblewrap and apk-tools.

| Phase | Name                          | Depends On | Key Deliverable                                                                   |
| ----- | ----------------------------- | ---------- | --------------------------------------------------------------------------------- |
| 1     | CLI Foundation                | —          | `yoe init`, `yoe config`, TOML parsing for all metadata types                     |
| 2     | Dependency Resolution         | 1          | DAG construction, topological sort, config propagation, `yoe desc`/`refs`/`graph` |
| 3     | Source Management             | 1          | `yoe source fetch/list/verify/clean`, content-addressed cache                     |
| 4     | Build Execution               | 2, 3       | `yoe build` with bubblewrap isolation, build step execution                       |
| 5     | Package Creation & Repository | 4          | APK package creation, `yoe repo` commands, local repository                       |
| 6     | Image Assembly                | 5          | Image recipe builds — rootfs via apk, overlays, disk image generation             |
| 7     | Device Interaction            | 6          | `yoe flash`, `yoe run` (QEMU with KVM)                                            |
| 8     | TUI                           | 2          | `yoe tui` — Bubble Tea interactive interface                                      |
| 9     | Bootstrap                     | 5          | `yoe bootstrap stage0/stage1` — self-hosting toolchain                            |

---

## Phase 1: CLI Foundation

**Goal:** Establish the Go project, stdlib CLI with switch/case dispatch (brun
pattern), TOML metadata parsing for all types, `yoe init` scaffolding, and
`yoe config` — the skeleton everything else builds on.

### File Structure

```
cmd/yoe/main.go                    — entry point, switch/case command dispatch, printUsage()
internal/config/project.go         — project discovery (find distro.toml)
internal/config/distro.go          — distro.toml parsing + types
internal/config/machine.go         — machine TOML parsing + types
internal/config/recipe.go          — recipe TOML parsing + types (package + image)
internal/config/partition.go       — partition layout TOML parsing + types
internal/config/loader.go          — load all config from a project tree
internal/init.go                   — yoe init logic
internal/clean.go                  — yoe clean logic
go.mod
go.sum
testdata/valid-project/            — test fixture: complete valid project
testdata/minimal-project/          — test fixture: minimal valid project
testdata/invalid-project/          — test fixture: various invalid configs
```

---

### Task 1: Go Module and CLI Skeleton

**Files:**

- Create: `go.mod`
- Create: `cmd/yoe/main.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /scratch4/yoe/yoe-ng
go mod init github.com/YoeDistro/yoe-ng
```

- [ ] **Step 2: Write the entry point with command dispatch**

Create `cmd/yoe/main.go`:

```go
package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "init":
		cmdInit(args)
	case "config":
		cmdConfig(args)
	case "clean":
		cmdClean(args)
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s COMMAND [OPTIONS]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Yoe-NG embedded Linux distribution builder\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  init <project-dir>      Create a new Yoe-NG project\n")
	fmt.Fprintf(os.Stderr, "  build [recipes...]      Build recipes (packages and images)\n")
	fmt.Fprintf(os.Stderr, "  flash <device>          Write an image to a device/SD card\n")
	fmt.Fprintf(os.Stderr, "  run                     Run an image in QEMU\n")
	fmt.Fprintf(os.Stderr, "  repo                    Manage the local apk package repository\n")
	fmt.Fprintf(os.Stderr, "  cache                   Manage the build cache (local and remote)\n")
	fmt.Fprintf(os.Stderr, "  source                  Download and manage source archives/repos\n")
	fmt.Fprintf(os.Stderr, "  config                  View and edit project configuration\n")
	fmt.Fprintf(os.Stderr, "  desc <recipe>           Describe a recipe or target\n")
	fmt.Fprintf(os.Stderr, "  refs <recipe>           Show reverse dependencies\n")
	fmt.Fprintf(os.Stderr, "  graph                   Visualize the dependency DAG\n")
	fmt.Fprintf(os.Stderr, "  tui                     Launch the interactive TUI\n")
	fmt.Fprintf(os.Stderr, "  clean                   Remove build artifacts\n")
	fmt.Fprintf(os.Stderr, "  version                 Display version information\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Init Options:\n")
	fmt.Fprintf(os.Stderr, "  -machine <name>         Initial machine to configure\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Config Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  config show             Show current configuration\n")
	fmt.Fprintf(os.Stderr, "  config set <key> <val>  Set a configuration value\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Clean Options:\n")
	fmt.Fprintf(os.Stderr, "  -all                    Remove everything (build dirs, packages, sources)\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Environment Variables:\n")
	fmt.Fprintf(os.Stderr, "  YOE_PROJECT             Project directory (default: cwd)\n")
	fmt.Fprintf(os.Stderr, "  YOE_CACHE               Cache directory (default: ~/.cache/yoe-ng)\n")
	fmt.Fprintf(os.Stderr, "  YOE_LOG                 Log level: debug, info, warn, error (default: info)\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  %s init my-project --machine beaglebone-black\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s build openssh\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s build base-image --machine raspberrypi4\n", os.Args[0])
}

// Stub command handlers — implemented in subsequent tasks

func cmdInit(args []string) {
	fmt.Fprintf(os.Stderr, "init: not yet implemented\n")
	os.Exit(1)
}

func cmdConfig(args []string) {
	fmt.Fprintf(os.Stderr, "config: not yet implemented\n")
	os.Exit(1)
}

func cmdClean(args []string) {
	fmt.Fprintf(os.Stderr, "clean: not yet implemented\n")
	os.Exit(1)
}
```

- [ ] **Step 3: Build and run**

```bash
go build -o yoe ./cmd/yoe
./yoe
./yoe version
```

Expected: Usage text on bare `yoe`, "dev" on `yoe version`.

- [ ] **Step 4: Commit**

```bash
git add go.mod cmd/
git commit -m "feat: initialize Go module with stdlib CLI skeleton"
```

---

### Task 2: Distro Configuration Parsing

**Files:**

- Create: `internal/config/distro.go`
- Create: `internal/config/distro_test.go`
- Create: `testdata/valid-project/distro.toml`

- [ ] **Step 1: Create test fixture**

Create `testdata/valid-project/distro.toml`:

```toml
[distro]
name = "test-distro"
version = "0.1.0"
description = "Test distribution"

[defaults]
machine = "qemu-arm64"
image = "base-image"

[repository]
path = "/var/cache/yoe-ng/repo"

[cache]
path = "/var/cache/yoe-ng/build"

[[cache.remote]]
name = "team"
type = "s3"
bucket = "yoe-cache"
endpoint = "https://minio.internal:9000"
region = "us-east-1"

[sources]
go-proxy = "https://proxy.golang.org"
```

- [ ] **Step 2: Write the failing test**

Create `internal/config/distro_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"
)

func TestParseDistroConfig(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-project", "distro.toml")
	distro, err := ParseDistroConfig(path)
	if err != nil {
		t.Fatalf("ParseDistroConfig(%q): %v", path, err)
	}

	if distro.Distro.Name != "test-distro" {
		t.Errorf("Name = %q, want %q", distro.Distro.Name, "test-distro")
	}
	if distro.Distro.Version != "0.1.0" {
		t.Errorf("Version = %q, want %q", distro.Distro.Version, "0.1.0")
	}
	if distro.Defaults.Machine != "qemu-arm64" {
		t.Errorf("Defaults.Machine = %q, want %q", distro.Defaults.Machine, "qemu-arm64")
	}
	if distro.Defaults.Image != "base" {
		t.Errorf("Defaults.Image = %q, want %q", distro.Defaults.Image, "base")
	}
	if distro.Repository.Path != "/var/cache/yoe-ng/repo" {
		t.Errorf("Repository.Path = %q, want %q", distro.Repository.Path, "/var/cache/yoe-ng/repo")
	}
	if distro.Cache.Path != "/var/cache/yoe-ng/build" {
		t.Errorf("Cache.Path = %q, want %q", distro.Cache.Path, "/var/cache/yoe-ng/build")
	}
	if distro.Sources.GoProxy != "https://proxy.golang.org" {
		t.Errorf("Sources.GoProxy = %q, want %q", distro.Sources.GoProxy, "https://proxy.golang.org")
	}
}

func TestParseDistroConfig_MissingFile(t *testing.T) {
	_, err := ParseDistroConfig("nonexistent.toml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseDistroConfig_RequiredFields(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "invalid-project", "empty-distro.toml")
	_, err := ParseDistroConfig(path)
	if err == nil {
		t.Fatal("expected error for empty distro name, got nil")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/config/ -run TestParseDistroConfig -v
```

Expected: FAIL — `ParseDistroConfig` not defined.

- [ ] **Step 4: Write the implementation**

Install TOML library:

```bash
go get github.com/BurntSushi/toml@latest
```

Create `internal/config/distro.go`:

```go
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type DistroConfig struct {
	Distro     DistroInfo          `toml:"distro"`
	Defaults   DistroDefaults      `toml:"defaults"`
	Repository RepoConfig          `toml:"repository"`
	Cache      CacheConfig         `toml:"cache"`
	Sources    SourcesConfig       `toml:"sources"`
	Layers     map[string]LayerRef `toml:"layers"`
}

type DistroInfo struct {
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Description string `toml:"description"`
}

type DistroDefaults struct {
	Machine string `toml:"machine"`
	Image   string `toml:"image"`
}

type RepoConfig struct {
	Path   string `toml:"path"`
	Remote string `toml:"remote"`
}

type CacheConfig struct {
	Path      string              `toml:"path"`
	Remote    []CacheRemoteConfig `toml:"remote"`
	Retention CacheRetention      `toml:"retention"`
	Signing   CacheSigningConfig  `toml:"signing"`
}

type CacheRemoteConfig struct {
	Name     string `toml:"name"`
	Type     string `toml:"type"`     // "s3"
	Bucket   string `toml:"bucket"`
	Endpoint string `toml:"endpoint"`
	Region   string `toml:"region"`
	Prefix   string `toml:"prefix"`
}

type CacheRetention struct {
	Days int `toml:"days"`
}

type CacheSigningConfig struct {
	PublicKey  string `toml:"public-key"`
	PrivateKey string `toml:"private-key"`
}

type SourcesConfig struct {
	GoProxy       string `toml:"go-proxy"`
	CargoRegistry string `toml:"cargo-registry"`
	NpmRegistry   string `toml:"npm-registry"`
}

type LayerRef struct {
	URL string `toml:"url"`
	Ref string `toml:"ref"`
}

func ParseDistroConfig(path string) (*DistroConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading distro config: %w", err)
	}

	var cfg DistroConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing distro config %s: %w", path, err)
	}

	if cfg.Distro.Name == "" {
		return nil, fmt.Errorf("distro config %s: distro.name is required", path)
	}

	return &cfg, nil
}
```

- [ ] **Step 5: Create invalid test fixture**

Create `testdata/invalid-project/empty-distro.toml`:

```toml
[distro]
name = ""
version = "0.1.0"
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/config/ -run TestParseDistroConfig -v
```

Expected: All 3 tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config/distro.go internal/config/distro_test.go testdata/
git commit -m "feat: add distro.toml parsing with validation"
```

---

### Task 3: Machine Configuration Parsing

**Files:**

- Create: `internal/config/machine.go`
- Create: `internal/config/machine_test.go`
- Create: `testdata/valid-project/machines/beaglebone-black.toml`
- Create: `testdata/valid-project/machines/qemu-x86_64.toml`

- [ ] **Step 1: Create test fixtures**

Create `testdata/valid-project/machines/beaglebone-black.toml`:

```toml
[machine]
name = "beaglebone-black"
arch = "arm64"
description = "BeagleBone Black (AM3358)"

[kernel]
repo = "https://github.com/beagleboard/linux.git"
branch = "6.6"
defconfig = "bb.org_defconfig"
device-trees = ["am335x-boneblack.dtb"]

[bootloader]
type = "u-boot"
repo = "https://github.com/beagleboard/u-boot.git"
branch = "v2024.01"
defconfig = "am335x_evm_defconfig"

[image]
partition-layout = "partitions/bbb.toml"
```

Create `testdata/valid-project/machines/qemu-x86_64.toml`:

```toml
[machine]
name = "qemu-x86_64"
arch = "x86_64"

[kernel]
recipe = "linux-qemu"
cmdline = "console=ttyS0 root=/dev/vda2 rw"

[machine.qemu]
machine = "q35"
cpu = "host"
memory = "1G"
firmware = "ovmf"
display = "none"
```

- [ ] **Step 2: Write the failing test**

Create `internal/config/machine_test.go`:

```go
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
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/config/ -run TestParseMachineConfig -v
```

Expected: FAIL — `ParseMachineConfig` not defined.

- [ ] **Step 4: Write the implementation**

Create `internal/config/machine.go`:

```go
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

var validArchitectures = map[string]bool{
	"arm":     true,
	"arm64":   true,
	"riscv64": true,
	"x86_64":  true,
}

type MachineConfig struct {
	Machine    MachineInfo  `toml:"machine"`
	Kernel     KernelConfig `toml:"kernel"`
	Bootloader BootConfig   `toml:"bootloader"`
	Image      MachineImage `toml:"image"`
}

type MachineInfo struct {
	Name        string      `toml:"name"`
	Arch        string      `toml:"arch"`
	Description string      `toml:"description"`
	QEMU        *QEMUConfig `toml:"qemu"`
}

type KernelConfig struct {
	Repo        string   `toml:"repo"`
	Branch      string   `toml:"branch"`
	Tag         string   `toml:"tag"`
	Defconfig   string   `toml:"defconfig"`
	DeviceTrees []string `toml:"device-trees"`
	Recipe      string   `toml:"recipe"`
	Cmdline     string   `toml:"cmdline"`
}

type BootConfig struct {
	Type      string `toml:"type"`
	Repo      string `toml:"repo"`
	Branch    string `toml:"branch"`
	Defconfig string `toml:"defconfig"`
}

type MachineImage struct {
	PartitionLayout string `toml:"partition-layout"`
}

type QEMUConfig struct {
	Machine  string `toml:"machine"`
	CPU      string `toml:"cpu"`
	Memory   string `toml:"memory"`
	Firmware string `toml:"firmware"`
	Display  string `toml:"display"`
}

func ParseMachineConfig(path string) (*MachineConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading machine config: %w", err)
	}

	var cfg MachineConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing machine config %s: %w", path, err)
	}

	if cfg.Machine.Name == "" {
		return nil, fmt.Errorf("machine config %s: machine.name is required", path)
	}
	if cfg.Machine.Arch == "" {
		return nil, fmt.Errorf("machine config %s: machine.arch is required", path)
	}
	if !validArchitectures[cfg.Machine.Arch] {
		return nil, fmt.Errorf("machine config %s: invalid arch %q (valid: arm, arm64, riscv64, x86_64)", path, cfg.Machine.Arch)
	}

	return &cfg, nil
}
```

- [ ] **Step 5: Create invalid test fixture**

Create `testdata/invalid-project/bad-arch-machine.toml`:

```toml
[machine]
name = "bad-machine"
arch = "mips"
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/config/ -run TestParseMachineConfig -v
```

Expected: All 3 tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config/machine.go internal/config/machine_test.go testdata/
git commit -m "feat: add machine TOML parsing with arch validation"
```

---

### Task 4: Recipe Configuration Parsing

**Files:**

- Create: `internal/config/recipe.go`
- Create: `internal/config/recipe_test.go`
- Create: `testdata/valid-project/recipes/openssh.toml`
- Create: `testdata/valid-project/recipes/myapp.toml`

- [ ] **Step 1: Create test fixtures**

Create `testdata/valid-project/recipes/openssh.toml`:

```toml
[recipe]
name = "openssh"
version = "9.6p1"
description = "OpenSSH client and server"
license = "BSD"

[source]
url = "https://cdn.openbsd.org/pub/OpenBSD/OpenSSH/portable/openssh-9.6p1.tar.gz"
sha256 = "aaaa1111bbbb2222cccc3333dddd4444eeee5555ffff6666"

[depends]
build = ["zlib", "openssl"]
runtime = ["zlib", "openssl"]

[build]
steps = [
    "./configure --prefix=$PREFIX --sysconfdir=/etc/ssh",
    "make -j$NPROC",
    "make DESTDIR=$DESTDIR install",
]

[package]
units = ["sshd.service"]
conffiles = ["/etc/ssh/sshd_config"]
```

Create `testdata/valid-project/recipes/myapp.toml`:

```toml
[recipe]
name = "myapp"
version = "1.2.3"
description = "Edge data collection service"
license = "Apache-2.0"
language = "go"

[source]
repo = "https://github.com/example/myapp.git"
tag = "v1.2.3"

[depends]
build = []
runtime = []

[build]
command = "go build -o $DESTDIR/usr/bin/myapp ./cmd/myapp"

[package]
units = ["myapp.service"]
conffiles = ["/etc/myapp/config.toml"]
```

- [ ] **Step 2: Write the failing test**

Create `internal/config/recipe_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"
)

func TestParseRecipeConfig_SystemPackage(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-project", "recipes", "openssh.toml")
	recipe, err := ParseRecipeConfig(path)
	if err != nil {
		t.Fatalf("ParseRecipeConfig(%q): %v", path, err)
	}

	if recipe.Recipe.Name != "openssh" {
		t.Errorf("Name = %q, want %q", recipe.Recipe.Name, "openssh")
	}
	if recipe.Recipe.Version != "9.6p1" {
		t.Errorf("Version = %q, want %q", recipe.Recipe.Version, "9.6p1")
	}
	if recipe.Source.URL != "https://cdn.openbsd.org/pub/OpenBSD/OpenSSH/portable/openssh-9.6p1.tar.gz" {
		t.Errorf("Source.URL = %q, want correct URL", recipe.Source.URL)
	}
	if len(recipe.Depends.Build) != 2 {
		t.Errorf("Depends.Build has %d entries, want 2", len(recipe.Depends.Build))
	}
	if len(recipe.Build.Steps) != 3 {
		t.Errorf("Build.Steps has %d entries, want 3", len(recipe.Build.Steps))
	}
	if recipe.Build.Command != "" {
		t.Errorf("Build.Command = %q, want empty for steps-based recipe", recipe.Build.Command)
	}
}

func TestParseRecipeConfig_AppPackage(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-project", "recipes", "myapp.toml")
	recipe, err := ParseRecipeConfig(path)
	if err != nil {
		t.Fatalf("ParseRecipeConfig(%q): %v", path, err)
	}

	if recipe.Recipe.Language != "go" {
		t.Errorf("Language = %q, want %q", recipe.Recipe.Language, "go")
	}
	if recipe.Source.Repo != "https://github.com/example/myapp.git" {
		t.Errorf("Source.Repo = %q, want correct URL", recipe.Source.Repo)
	}
	if recipe.Source.Tag != "v1.2.3" {
		t.Errorf("Source.Tag = %q, want %q", recipe.Source.Tag, "v1.2.3")
	}
	if recipe.Build.Command == "" {
		t.Error("Build.Command is empty, want a command string")
	}
}

func TestParseRecipeConfig_NoBuildMethod(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "invalid-project", "no-build-recipe.toml")
	_, err := ParseRecipeConfig(path)
	if err == nil {
		t.Fatal("expected error for recipe with no build steps or command, got nil")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/config/ -run TestParseRecipeConfig -v
```

Expected: FAIL — `ParseRecipeConfig` not defined.

- [ ] **Step 4: Write the implementation**

Create `internal/config/recipe.go`:

```go
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type RecipeConfig struct {
	Recipe  RecipeInfo    `toml:"recipe"`
	Source  SourceConfig  `toml:"source"`
	Depends DependsConfig `toml:"depends"`
	Build   BuildConfig   `toml:"build"`
	Package PackageConfig `toml:"package"`
	Image   ImageSection  `toml:"image"` // only for type = "image"
}

type ImageSection struct {
	Hostname        string        `toml:"hostname"`
	Timezone        string        `toml:"timezone"`
	Locale          string        `toml:"locale"`
	PartitionLayout string        `toml:"partition-layout"`
	Services        ImageServices `toml:"services"`
}

type ImageServices struct {
	Enable []string `toml:"enable"`
}

type RecipeInfo struct {
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Type        string `toml:"type"`        // "package" (default) or "image"
	Description string `toml:"description"`
	License     string `toml:"license"`
	Language    string `toml:"language"`
	Extends     string `toml:"extends"`     // for image inheritance
}

type SourceConfig struct {
	URL    string `toml:"url"`
	SHA256 string `toml:"sha256"`
	Repo   string `toml:"repo"`
	Tag    string `toml:"tag"`
	Branch string `toml:"branch"`
}

type DependsConfig struct {
	Build   []string `toml:"build"`
	Runtime []string `toml:"runtime"`
}

type BuildConfig struct {
	Steps   []string `toml:"steps"`
	Command string   `toml:"command"`
}

type PackageConfig struct {
	Units       []string          `toml:"units"`
	Conffiles   []string          `toml:"conffiles"`
	Environment map[string]string `toml:"environment"`
}

func ParseRecipeConfig(path string) (*RecipeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading recipe config: %w", err)
	}

	var cfg RecipeConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing recipe config %s: %w", path, err)
	}

	if cfg.Recipe.Name == "" {
		return nil, fmt.Errorf("recipe config %s: recipe.name is required", path)
	}
	if cfg.Recipe.Version == "" {
		return nil, fmt.Errorf("recipe config %s: recipe.version is required", path)
	}
	// Default type to "package"
	if cfg.Recipe.Type == "" {
		cfg.Recipe.Type = "package"
	}
	if cfg.Recipe.Type != "package" && cfg.Recipe.Type != "image" {
		return nil, fmt.Errorf("recipe config %s: type must be 'package' or 'image', got %q", path, cfg.Recipe.Type)
	}
	// Package recipes require build steps; image recipes do not
	if cfg.Recipe.Type == "package" && len(cfg.Build.Steps) == 0 && cfg.Build.Command == "" {
		return nil, fmt.Errorf("recipe config %s: build.steps or build.command is required for package recipes", path)
	}

	return &cfg, nil
}
```

- [ ] **Step 5: Create invalid test fixture**

Create `testdata/invalid-project/no-build-recipe.toml`:

```toml
[recipe]
name = "broken"
version = "1.0.0"

[source]
url = "https://example.com/broken.tar.gz"

[depends]
build = []
runtime = []
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/config/ -run TestParseRecipeConfig -v
```

Expected: All 3 tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config/recipe.go internal/config/recipe_test.go testdata/
git commit -m "feat: add recipe TOML parsing for system and app packages"
```

---

### Task 5: Image Recipe and Partition Configuration Parsing

**Files:**

- Create: `internal/config/partition.go`
- Create: `internal/config/partition_test.go`
- Create: `testdata/valid-project/recipes/base-image.toml`
- Create: `testdata/valid-project/partitions/bbb.toml`

- [ ] **Step 1: Create test fixtures**

Create `testdata/valid-project/recipes/base-image.toml`:

```toml
[recipe]
name = "base-image"
version = "1.0.0"
type = "image"
description = "Minimal bootable system"

[depends]
runtime = ["openssh", "networkmanager", "myapp"]

[image]
hostname = "yoe"
timezone = "UTC"
locale = "en_US.UTF-8"
partition-layout = "partitions/bbb.toml"

[image.services]
enable = ["sshd", "NetworkManager", "myapp"]
```

Create `testdata/valid-project/partitions/bbb.toml`:

```toml
[disk]
type = "gpt"

[[partition]]
label = "boot"
type = "vfat"
size = "64M"
contents = ["MLO", "u-boot.img", "zImage", "*.dtb"]

[[partition]]
label = "rootfs"
type = "ext4"
size = "fill"
root = true
```

- [ ] **Step 2: Write the failing tests**

Add to `internal/config/recipe_test.go`:

```go
func TestParseRecipeConfig_ImageRecipe(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-project", "recipes", "base-image.toml")
	recipe, err := ParseRecipeConfig(path)
	if err != nil {
		t.Fatalf("ParseRecipeConfig(%q): %v", path, err)
	}

	if recipe.Recipe.Type != "image" {
		t.Errorf("Type = %q, want %q", recipe.Recipe.Type, "image")
	}
	if recipe.Recipe.Name != "base-image" {
		t.Errorf("Name = %q, want %q", recipe.Recipe.Name, "base-image")
	}
	if len(recipe.Depends.Runtime) != 3 {
		t.Errorf("Depends.Runtime has %d entries, want 3", len(recipe.Depends.Runtime))
	}
	if recipe.Image.Hostname != "yoe" {
		t.Errorf("Image.Hostname = %q, want %q", recipe.Image.Hostname, "yoe")
	}
	if len(recipe.Image.Services.Enable) != 3 {
		t.Errorf("Image.Services.Enable has %d entries, want 3", len(recipe.Image.Services.Enable))
	}
}
```

Create `internal/config/partition_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"
)

func TestParsePartitionConfig(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-project", "partitions", "bbb.toml")
	part, err := ParsePartitionConfig(path)
	if err != nil {
		t.Fatalf("ParsePartitionConfig(%q): %v", path, err)
	}

	if part.Disk.Type != "gpt" {
		t.Errorf("Disk.Type = %q, want %q", part.Disk.Type, "gpt")
	}
	if len(part.Partitions) != 2 {
		t.Errorf("got %d partitions, want 2", len(part.Partitions))
	}
	if part.Partitions[0].Label != "boot" {
		t.Errorf("Partition[0].Label = %q, want %q", part.Partitions[0].Label, "boot")
	}
	if part.Partitions[1].Root != true {
		t.Error("Partition[1].Root = false, want true")
	}
}

func TestParsePartitionConfig_NoRootPartition(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "invalid-project", "no-root-partition.toml")
	_, err := ParsePartitionConfig(path)
	if err == nil {
		t.Fatal("expected error for partition layout with no root partition, got nil")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/config/ -run "TestParseRecipeConfig_ImageRecipe|TestParsePartitionConfig" -v
```

Expected: FAIL — `ParsePartitionConfig` not defined (image recipe test should
already pass from Task 4's implementation since `ImageSection` and
`ImageServices` types were added to recipe.go).

- [ ] **Step 4: Write the partition implementation**

Create `internal/config/partition.go`:

```go
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type PartitionConfig struct {
	Disk       DiskConfig  `toml:"disk"`
	Partitions []Partition `toml:"partition"`
}

type DiskConfig struct {
	Type string `toml:"type"`
}

type Partition struct {
	Label    string   `toml:"label"`
	Type     string   `toml:"type"`
	Size     string   `toml:"size"`
	Root     bool     `toml:"root"`
	Contents []string `toml:"contents"`
}

func ParsePartitionConfig(path string) (*PartitionConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading partition config: %w", err)
	}

	var cfg PartitionConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing partition config %s: %w", path, err)
	}

	if cfg.Disk.Type == "" {
		return nil, fmt.Errorf("partition config %s: disk.type is required", path)
	}
	if cfg.Disk.Type != "gpt" && cfg.Disk.Type != "mbr" {
		return nil, fmt.Errorf("partition config %s: disk.type must be 'gpt' or 'mbr', got %q", path, cfg.Disk.Type)
	}

	hasRoot := false
	for _, p := range cfg.Partitions {
		if p.Root {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		return nil, fmt.Errorf("partition config %s: at least one partition must have root = true", path)
	}

	return &cfg, nil
}
```

- [ ] **Step 5: Create invalid test fixture**

Create `testdata/invalid-project/no-root-partition.toml`:

```toml
[disk]
type = "gpt"

[[partition]]
label = "boot"
type = "vfat"
size = "64M"
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/config/ -run "TestParseRecipeConfig_ImageRecipe|TestParsePartitionConfig" -v
```

Expected: All tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config/partition.go internal/config/partition_test.go internal/config/recipe_test.go testdata/
git commit -m "feat: add image recipe support and partition TOML parsing"
```

---

### Task 6: Project Loader

**Files:**

- Create: `internal/config/loader.go`
- Create: `internal/config/loader_test.go`
- Create: `internal/config/project.go`
- Create: `internal/config/project_test.go`

- [ ] **Step 1: Write the failing test for project discovery**

Create `internal/config/project_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"
)

func TestFindProjectRoot(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "valid-project")
	root, err := FindProjectRoot(dir)
	if err != nil {
		t.Fatalf("FindProjectRoot(%q): %v", dir, err)
	}
	absDir, _ := filepath.Abs(dir)
	if root != absDir {
		t.Errorf("FindProjectRoot = %q, want %q", root, absDir)
	}
}

func TestFindProjectRoot_NotFound(t *testing.T) {
	_, err := FindProjectRoot("/tmp")
	if err == nil {
		t.Fatal("expected error when no distro.toml found, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/ -run TestFindProjectRoot -v
```

Expected: FAIL — `FindProjectRoot` not defined.

- [ ] **Step 3: Write the project discovery implementation**

Create `internal/config/project.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func FindProjectRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	for {
		candidate := filepath.Join(dir, "distro.toml")
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no distro.toml found in %s or any parent directory", startDir)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/config/ -run TestFindProjectRoot -v
```

Expected: PASS.

- [ ] **Step 5: Write the failing test for project loader**

Create `internal/config/loader_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"
)

func TestLoadProject(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "valid-project")
	project, err := LoadProject(dir)
	if err != nil {
		t.Fatalf("LoadProject(%q): %v", dir, err)
	}

	if project.Distro.Distro.Name != "test-distro" {
		t.Errorf("Distro.Name = %q, want %q", project.Distro.Distro.Name, "test-distro")
	}
	if len(project.Machines) == 0 {
		t.Error("expected at least one machine, got 0")
	}
	if _, ok := project.Machines["beaglebone-black"]; !ok {
		t.Error("expected machine 'beaglebone-black' to be loaded")
	}
	if len(project.Recipes) == 0 {
		t.Error("expected at least one recipe, got 0")
	}
	if _, ok := project.Recipes["openssh"]; !ok {
		t.Error("expected recipe 'openssh' to be loaded")
	}
	if len(project.Images) == 0 {
		t.Error("expected at least one image, got 0")
	}
	if r, ok := project.Recipes["base-image"]; !ok {
		t.Error("expected recipe 'base-image' to be loaded")
	} else if r.Recipe.Type != "image" {
		t.Errorf("base-image type = %q, want %q", r.Recipe.Type, "image")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

```bash
go test ./internal/config/ -run TestLoadProject -v
```

Expected: FAIL — `LoadProject` not defined.

- [ ] **Step 7: Write the loader implementation**

Create `internal/config/loader.go`:

```go
package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Project struct {
	Root       string
	Distro     *DistroConfig
	Machines   map[string]*MachineConfig
	Recipes    map[string]*RecipeConfig    // includes both package and image recipes
	Partitions map[string]*PartitionConfig
}

func LoadProject(dir string) (*Project, error) {
	root, err := FindProjectRoot(dir)
	if err != nil {
		return nil, err
	}

	distro, err := ParseDistroConfig(filepath.Join(root, "distro.toml"))
	if err != nil {
		return nil, err
	}

	project := &Project{
		Root:       root,
		Distro:     distro,
		Machines:   make(map[string]*MachineConfig),
		Recipes:    make(map[string]*RecipeConfig),
		Partitions: make(map[string]*PartitionConfig),
	}

	if err := project.loadDir("machines", func(path string) error {
		m, err := ParseMachineConfig(path)
		if err != nil {
			return err
		}
		project.Machines[m.Machine.Name] = m
		return nil
	}); err != nil {
		return nil, err
	}

	// All recipes (package and image types) live in recipes/
	if err := project.loadDir("recipes", func(path string) error {
		r, err := ParseRecipeConfig(path)
		if err != nil {
			return err
		}
		project.Recipes[r.Recipe.Name] = r
		return nil
	}); err != nil {
		return nil, err
	}

	if err := project.loadDir("partitions", func(path string) error {
		p, err := ParsePartitionConfig(path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".toml")
		project.Partitions[name] = p
		return nil
	}); err != nil {
		return nil, err
	}

	return project, nil
}

func (p *Project) loadDir(subdir string, load func(string) error) error {
	pattern := filepath.Join(p.Root, subdir, "*.toml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("globbing %s: %w", pattern, err)
	}
	for _, path := range matches {
		if err := load(path); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 8: Run tests to verify they pass**

```bash
go test ./internal/config/ -run "TestFindProjectRoot|TestLoadProject" -v
```

Expected: All tests PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/config/project.go internal/config/project_test.go internal/config/loader.go internal/config/loader_test.go
git commit -m "feat: add project discovery and full config loader"
```

---

### Task 7: `yoe init` Command

**Files:**

- Create: `internal/init.go`
- Create: `internal/init_test.go`
- Modify: `cmd/yoe/main.go` — wire up cmdInit

- [ ] **Step 1: Write the failing test**

Create `internal/init_test.go`:

```go
package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunInit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-project")

	if err := RunInit(dir, ""); err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	for _, path := range []string{
		"distro.toml",
		"machines",
		"recipes",
		"partitions",
		"overlays",
	} {
		full := filepath.Join(dir, path)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after init", path)
		}
	}
}

func TestRunInit_WithMachine(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-project")

	if err := RunInit(dir, "qemu-x86_64"); err != nil {
		t.Fatalf("RunInit with machine: %v", err)
	}

	machineFile := filepath.Join(dir, "machines", "qemu-x86_64.toml")
	if _, err := os.Stat(machineFile); os.IsNotExist(err) {
		t.Errorf("expected machine file %s to exist", machineFile)
	}
}

func TestRunInit_ExistingProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "distro.toml"), []byte("[distro]\nname = \"exists\"\n"), 0644)

	if err := RunInit(dir, ""); err == nil {
		t.Fatal("expected error when init into existing project, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/ -run TestRunInit -v
```

Expected: FAIL — `RunInit` not defined.

- [ ] **Step 3: Write the implementation**

Create `internal/init.go`:

```go
package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

var distroTemplate = `[distro]
name = "{{.Name}}"
version = "0.1.0"
description = ""

[defaults]
machine = "{{.Machine}}"
image = "base"

[repository]
path = "/var/cache/yoe-ng/repo"

[cache]
path = "/var/cache/yoe-ng/build"

[sources]
go-proxy = "https://proxy.golang.org"
`

var qemuMachineTemplate = `[machine]
name = "{{.Name}}"
arch = "{{.Arch}}"

[kernel]
recipe = "linux-qemu"
cmdline = "console={{.Console}} root=/dev/vda2 rw"

[machine.qemu]
machine = "{{.QEMUMachine}}"
cpu = "host"
memory = "1G"
firmware = "{{.Firmware}}"
display = "none"
`

func RunInit(projectDir string, machine string) error {
	if _, err := os.Stat(filepath.Join(projectDir, "distro.toml")); err == nil {
		return fmt.Errorf("project already exists at %s (distro.toml found)", projectDir)
	}

	dirs := []string{"machines", "recipes", "partitions", "overlays"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(projectDir, dir), 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	name := filepath.Base(projectDir)
	defaultMachine := machine
	if defaultMachine == "" {
		defaultMachine = "qemu-x86_64"
	}

	tmpl, err := template.New("distro").Parse(distroTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	f, err := os.Create(filepath.Join(projectDir, "distro.toml"))
	if err != nil {
		return fmt.Errorf("creating distro.toml: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, map[string]string{
		"Name":    name,
		"Machine": defaultMachine,
	}); err != nil {
		return fmt.Errorf("writing distro.toml: %w", err)
	}

	if machine != "" {
		if err := createMachineFile(projectDir, machine); err != nil {
			return err
		}
	}

	fmt.Printf("Created Yoe-NG project at %s\n", projectDir)
	return nil
}

func createMachineFile(projectDir, name string) error {
	type machineData struct {
		Name, Arch, Console, QEMUMachine, Firmware string
	}

	data := machineData{Name: name}

	switch {
	case name == "qemu-x86_64" || name == "x86_64":
		data.Arch = "x86_64"
		data.Console = "ttyS0"
		data.QEMUMachine = "q35"
		data.Firmware = "ovmf"
	case name == "qemu-arm64" || name == "aarch64":
		data.Arch = "arm64"
		data.Console = "ttyAMA0"
		data.QEMUMachine = "virt"
		data.Firmware = "aavmf"
	case name == "qemu-riscv64" || name == "riscv64":
		data.Arch = "riscv64"
		data.Console = "ttyS0"
		data.QEMUMachine = "virt"
		data.Firmware = "opensbi"
	default:
		path := filepath.Join(projectDir, "machines", name+".toml")
		content := fmt.Sprintf("[machine]\nname = %q\narch = \"arm64\"\ndescription = \"\"\n", name)
		return os.WriteFile(path, []byte(content), 0644)
	}

	tmpl, err := template.New("machine").Parse(qemuMachineTemplate)
	if err != nil {
		return fmt.Errorf("parsing machine template: %w", err)
	}

	path := filepath.Join(projectDir, "machines", name+".toml")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating machine file: %w", err)
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}
```

- [ ] **Step 4: Wire up cmdInit in main.go**

Replace the `cmdInit` stub in `cmd/yoe/main.go`:

```go
func cmdInit(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s init <project-dir> [-machine <name>]\n", os.Args[0])
		os.Exit(1)
	}

	projectDir := args[0]
	machine := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-machine":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: -machine requires a name\n")
				os.Exit(1)
			}
			machine = args[i+1]
			i++
		default:
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	if err := yoe.RunInit(projectDir, machine); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

Add the import: `yoe "github.com/YoeDistro/yoe-ng/internal"`

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/ -run TestRunInit -v
```

Expected: All 3 tests PASS.

- [ ] **Step 6: Build and smoke test**

```bash
go build -o yoe ./cmd/yoe
./yoe init /tmp/yoe-test -machine qemu-x86_64
ls /tmp/yoe-test/
cat /tmp/yoe-test/distro.toml
rm -rf /tmp/yoe-test
```

Expected: Project scaffolded with distro.toml and machine file.

- [ ] **Step 7: Commit**

```bash
git add internal/init.go internal/init_test.go cmd/yoe/main.go
git commit -m "feat: add yoe init command with project scaffolding"
```

---

### Task 8: `yoe config` Command

**Files:**

- Modify: `cmd/yoe/main.go` — wire up cmdConfig
- Create: `internal/configcmd.go`
- Create: `internal/configcmd_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/configcmd_test.go`:

```go
package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func setupTestProject(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "test-project")
	if err := RunInit(dir, "qemu-x86_64"); err != nil {
		t.Fatalf("init: %v", err)
	}
	return dir
}

func TestConfigShow(t *testing.T) {
	dir := setupTestProject(t)

	var buf bytes.Buffer
	if err := RunConfigShow(dir, &buf); err != nil {
		t.Fatalf("RunConfigShow: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("config show produced no output")
	}
}

func TestConfigSet(t *testing.T) {
	dir := setupTestProject(t)

	if err := RunConfigSet(dir, "defaults.machine", "beaglebone-black"); err != nil {
		t.Fatalf("RunConfigSet: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "distro.toml"))
	if err != nil {
		t.Fatalf("reading distro.toml: %v", err)
	}
	if !bytes.Contains(data, []byte("beaglebone-black")) {
		t.Error("distro.toml does not contain updated machine name")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/ -run TestConfig -v
```

Expected: FAIL — `RunConfigShow` not defined.

- [ ] **Step 3: Write the implementation**

Create `internal/configcmd.go`:

```go
package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/YoeDistro/yoe-ng/internal/config"
)

func RunConfigShow(dir string, w io.Writer) error {
	distro, err := config.ParseDistroConfig(filepath.Join(dir, "distro.toml"))
	if err != nil {
		return err
	}
	return toml.NewEncoder(w).Encode(distro)
}

func RunConfigSet(dir, key, value string) error {
	path := filepath.Join(dir, "distro.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading distro.toml: %w", err)
	}

	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing distro.toml: %w", err)
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("key must be in section.field format, got %q", key)
	}

	section, field := parts[0], parts[1]
	sectionMap, ok := raw[section].(map[string]interface{})
	if !ok {
		sectionMap = make(map[string]interface{})
		raw[section] = sectionMap
	}
	sectionMap[field] = value

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("writing distro.toml: %w", err)
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(raw)
}
```

- [ ] **Step 4: Wire up cmdConfig in main.go**

Replace the `cmdConfig` stub in `cmd/yoe/main.go`:

```go
func cmdConfig(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s config <show|set> [...]\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}

	switch args[0] {
	case "show":
		if err := yoe.RunConfigShow(dir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "set":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s config set <key> <value>\n", os.Args[0])
			os.Exit(1)
		}
		if err := yoe.RunConfigSet(dir, args[1], args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", args[0])
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/ -run TestConfig -v
```

Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/configcmd.go internal/configcmd_test.go cmd/yoe/main.go
git commit -m "feat: add yoe config show/set commands"
```

---

### Task 9: `yoe clean` Command

**Files:**

- Create: `internal/clean.go`
- Create: `internal/clean_test.go`
- Modify: `cmd/yoe/main.go` — wire up cmdClean

- [ ] **Step 1: Write the failing test**

Create `internal/clean_test.go`:

```go
package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunClean(t *testing.T) {
	dir := setupTestProject(t)

	buildDir := filepath.Join(dir, "build")
	os.MkdirAll(buildDir, 0755)
	os.WriteFile(filepath.Join(buildDir, "artifact.o"), []byte("fake"), 0644)

	if err := RunClean(dir, false); err != nil {
		t.Fatalf("RunClean: %v", err)
	}

	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		t.Error("expected build directory to be removed")
	}
}

func TestRunClean_All(t *testing.T) {
	dir := setupTestProject(t)

	for _, sub := range []string{"build", "packages", "sources"} {
		d := filepath.Join(dir, sub)
		os.MkdirAll(d, 0755)
		os.WriteFile(filepath.Join(d, "artifact"), []byte("fake"), 0644)
	}

	if err := RunClean(dir, true); err != nil {
		t.Fatalf("RunClean --all: %v", err)
	}

	for _, sub := range []string{"build", "packages", "sources"} {
		d := filepath.Join(dir, sub)
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("expected %s directory to be removed", sub)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/ -run TestRunClean -v
```

Expected: FAIL — `RunClean` not defined.

- [ ] **Step 3: Write the implementation**

Create `internal/clean.go`:

```go
package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

func RunClean(dir string, all bool) error {
	buildDir := filepath.Join(dir, "build")
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("removing build directory: %w", err)
	}
	fmt.Println("Removed build directory")

	if all {
		for _, subdir := range []string{"packages", "sources"} {
			path := filepath.Join(dir, subdir)
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("removing %s: %w", subdir, err)
			}
			fmt.Printf("Removed %s directory\n", subdir)
		}
	}

	return nil
}
```

- [ ] **Step 4: Wire up cmdClean in main.go**

Replace the `cmdClean` stub in `cmd/yoe/main.go`:

```go
func cmdClean(args []string) {
	all := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-all":
			all = true
		default:
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}

	if err := yoe.RunClean(dir, all); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/ -run TestRunClean -v
```

Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/clean.go internal/clean_test.go cmd/yoe/main.go
git commit -m "feat: add yoe clean command"
```

---

### Task 10: Run All Phase 1 Tests

- [ ] **Step 1: Run all tests**

```bash
go test ./... -v
```

Expected: All tests PASS.

- [ ] **Step 2: Build final binary**

```bash
go build -o yoe ./cmd/yoe && ./yoe
```

Expected: Help output showing all command stubs.

- [ ] **Step 3: Smoke test**

```bash
./yoe init /tmp/yoe-smoke-test -machine qemu-x86_64
./yoe version
YOE_PROJECT=/tmp/yoe-smoke-test ./yoe config show
YOE_PROJECT=/tmp/yoe-smoke-test ./yoe clean
rm -rf /tmp/yoe-smoke-test
```

Expected: All commands succeed.

- [ ] **Step 4: Commit any fixes from integration testing**

```bash
git add -A
git commit -m "fix: integration test fixes for phase 1"
```

(Skip if no fixes needed.)

---

## Phase 2: Dependency Resolution (detailed plan TBD)

**Goal:** Build the DAG engine — load all recipes, topologically sort, detect
cycles, propagate machine config through the graph.

**Key components:**

- `internal/resolve/dag.go` — directed acyclic graph data structure
- `internal/resolve/topo.go` — topological sort with cycle detection
- `internal/resolve/config.go` — config propagation (machine -> recipes,
  public_config)
- `internal/resolve/hash.go` — content hash computation for cache keys
- `cmd/yoe/main.go` — add `desc`, `refs`, `graph` commands to switch statement

**Depends on:** Phase 1 (config types and project loader)

---

## Phase 3: Source Management (detailed plan TBD)

**Goal:** Download, cache, and verify source archives and git repos.

**Key components:**

- `internal/source/fetch.go` — HTTP downloads + git clones
- `internal/source/cache.go` — content-addressed source cache
  ($YOE_CACHE/sources/)
- `internal/source/verify.go` — SHA256 verification
- `cmd/yoe/main.go` — add `source` command with subcommands to switch statement

**Depends on:** Phase 1 (recipe config for source URLs)

---

## Phase 4: Build Execution (detailed plan TBD)

**Goal:** Execute recipe build steps inside bubblewrap sandboxes.

**Key components:**

- `internal/build/sandbox.go` — bubblewrap wrapper (namespace setup, bind
  mounts)
- `internal/build/environment.go` — build environment assembly (apk install of
  build deps)
- `internal/build/executor.go` — build step execution with logging
- `internal/build/cache.go` — content-addressed build cache
- `cmd/yoe/main.go` — add `build` command to switch statement

**Depends on:** Phase 2 (DAG for build ordering), Phase 3 (source fetching)
**System requirements:** Linux with bubblewrap, apk-tools

---

## Phase 5: Package Creation, Repository & Cache (detailed plan TBD)

**Goal:** Create .apk packages from build output, manage a local repository, and
provide S3-compatible remote cache for sharing builds across CI/team.

**Key components:**

- `internal/packaging/apk.go` — .apk archive creation (.PKGINFO generation,
  tar.gz packaging)
- `internal/packaging/sign.go` — package signing
- `internal/repo/local.go` — local repository management (index, add, remove)
- `internal/repo/remote.go` — S3-compatible push/pull for repo
- `internal/cache/local.go` — local content-addressed build cache
- `internal/cache/remote.go` — S3-compatible remote cache (push/pull/gc)
- `internal/cache/sign.go` — cache package signing and verification
- `cmd/yoe/main.go` — add `repo` and `cache` commands to switch statement

**Depends on:** Phase 4 (build output in $DESTDIR)

---

## Phase 6: Image Assembly (detailed plan TBD)

**Goal:** Implement the image recipe build path — when `yoe build` encounters a
recipe with `type = "image"`, assemble a bootable disk image from packages
instead of compiling source code. No separate `yoe image` command; images are
built through the same `yoe build` pipeline.

**Key components:**

- `internal/image/rootfs.go` — rootfs creation via apk add --root
- `internal/image/configure.go` — hostname, timezone, locale, service enablement
- `internal/image/overlay.go` — overlay file copying
- `internal/image/disk.go` — partition table creation, filesystem formatting
- `internal/image/kernel.go` — kernel + bootloader installation
- `internal/build/executor.go` — extend to dispatch image recipes to image
  assembly instead of sandbox build

**Depends on:** Phase 5 (populated package repository) **System requirements:**
user namespaces (bubblewrap), mkfs.ext4, mkfs.vfat, systemd-repart

---

## Phase 7: Device Interaction (detailed plan TBD)

**Goal:** Flash images to devices and run in QEMU.

**Key components:**

- `internal/device/flash.go` — block device writing with safety checks (mounted?
  system disk?)
- `internal/device/qemu.go` — QEMU launch configuration (KVM, port forwarding,
  serial console)
- `cmd/yoe/main.go` — add `flash` and `run` commands to switch statement

**Depends on:** Phase 6 (disk images to flash/run)

---

## Phase 8: TUI (detailed plan TBD)

**Goal:** Interactive terminal UI for common workflows.

**Key components:**

- `internal/tui/app.go` — Bubble Tea application model
- `internal/tui/views/` — machine selector, build progress, log viewer
- `cmd/yoe/main.go` — add `tui` command to switch statement

**Depends on:** Phase 2 (recipe/machine listing), can start after Phase 2

---

## Phase 9: Bootstrap (detailed plan TBD)

**Goal:** Self-hosting bootstrap from Alpine — build glibc, gcc, binutils from
an existing toolchain, then rebuild with own toolchain.

**Key components:**

- `internal/bootstrap/stage0.go` — cross-pollination from Alpine
- `internal/bootstrap/stage1.go` — self-hosting rebuild
- `cmd/yoe/main.go` — add `bootstrap` command to switch statement
- Bootstrap recipe set: glibc, binutils, gcc, linux-headers, busybox, apk-tools,
  bubblewrap

**Depends on:** Phase 5 (package creation and repository) **This is the most
complex phase** — building a C library and compiler toolchain is non-trivial.
Consider starting with pre-built packages and implementing bootstrap last.
