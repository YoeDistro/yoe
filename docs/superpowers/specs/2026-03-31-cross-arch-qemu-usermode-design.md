# Cross-Architecture Builds via QEMU User-Mode Emulation

**Date:** 2026-03-31
**Status:** Draft

## Problem

Yoe-ng's "native builds only" model means you need matching hardware to build
for a target architecture. Building arm64 images requires an arm64 host. This is
a barrier for developers with x86 workstations targeting ARM boards.

## Solution

Use QEMU user-mode emulation (binfmt_misc) so Docker can run foreign-arch
containers transparently. When the target machine's arch differs from the host,
yoe builds and uses a container image for the target arch. Inside that container,
everything — gcc, make, configure scripts — runs as genuine ARM binaries,
emulated by the host kernel via binfmt_misc. The build executor, bwrap sandbox,
and unit definitions are unchanged.

## Design Decisions

- **Separate container image per arch** — `yoe-ng:11-arm64`, `yoe-ng:11-riscv64`
  alongside `yoe-ng:11` (host arch). Each is a genuine native image for that
  architecture, not a multi-arch image with emulation baked in. Same Dockerfile,
  different `--platform`.
- **Same lazy build pattern** — `EnsureImage` auto-builds the foreign-arch
  container on first use, same as the host container today. An ARM Alpine
  container build takes ~1-2 minutes under emulation (just `apk add`).
- **Explicit binfmt registration** — new `yoe container binfmt` command.
  Cross-arch builds detect missing binfmt and tell the user what to run. The
  command explains what it does and prompts for confirmation.
- **Arch from machine, not host** — the machine definition's `arch` field
  determines the target. `build.Arch()` continues to return host arch; a new
  concept of "target arch" flows from the machine through the build.
- **Direct kernel boot for arm64 QEMU** — simplest path, no firmware or
  bootloader needed. U-Boot/UEFI can be added later for real hardware.
- **QEMU system emulation for `yoe run`** — install `qemu-system-aarch64` and
  `qemu-system-riscv64` alongside the existing x86 binary. Auto-detect
  native (KVM) vs cross (TCG) at runtime.

## Architecture

```
Host (x86_64)
  └─ yoe CLI (native x86_64)
       ├─ Starlark eval, DAG resolve, source fetch  (native, no container)
       └─ RunInContainer(arch="arm64")
            └─ docker run --platform linux/arm64 yoe-ng:11-arm64
                 └─ bwrap sandbox
                      └─ ARM gcc, make, configure  (emulated via binfmt_misc)
```

## Changes Required

### 1. Target Arch Plumbing

**`build.Options`** — add `TargetArch` field (distinct from host `Arch`).

**`cmdBuild` / TUI** — when building an image unit, resolve the machine's arch
and set `TargetArch`. When building a standalone unit without a machine context,
`TargetArch` defaults to host arch (current behavior).

**Hash computation** — already includes `arch:` in the hash. Ensure it uses
`TargetArch` so x86_64 and arm64 builds of the same unit cache separately.

### 2. Container Image Per Arch

**Tag scheme:**

- Host arch: `yoe-ng:11` (unchanged)
- Foreign arch: `yoe-ng:11-arm64`, `yoe-ng:11-riscv64`

**`EnsureImage(arch string)`** — accept target arch parameter. When arch differs
from host, build with `docker buildx build --platform linux/<arch>`. The existing
Dockerfile works as-is — Alpine's `apk add` installs native packages for
whatever platform the container runs on.

**`RunInContainer`** — accept target arch. Select the correct container tag.
Add `--platform linux/<arch>` to the `docker run` args for foreign-arch
containers.

**`containerRunArgs`** — add `--platform` flag when target != host.

### 3. binfmt_misc Detection and Setup

**New command: `yoe container binfmt`**

```
$ yoe container binfmt
This will register QEMU user-mode emulation for foreign architectures
by running a privileged Docker container (tonistiigi/binfmt).

This enables building arm64 and riscv64 images on your x86_64 host.
The registration persists until reboot.

Proceed? (y/n) y
[yoe] registering binfmt_misc handlers...
Done.
```

Implementation: runs
`docker run --privileged --rm tonistiigi/binfmt --install arm64,riscv64`.

**Detection:** before building a cross-arch container, check
`/proc/sys/fs/binfmt_misc/qemu-<arch>`. If missing:

```
Error: binfmt_misc not registered for arm64.
Run 'yoe container binfmt' to enable cross-architecture builds.
```

### 4. Fix Hardcoded x86_64 in APK Packaging

**`internal/artifact/apk.go`** — `.PKGINFO` currently writes
`arch = x86_64`. Change to accept target arch parameter and write
`arch = <target_arch>`.

**`internal/image/rootfs.go`** — APK repo lookup currently checks `x86_64/`
subdirectory. Parameterize to use target arch.

**`internal/repo/`** — repo directory structure should use arch subdirectories:
`build/repo/<arch>/`.

### 5. QEMU System Emulation for `yoe run`

**Install additional QEMU binaries** — add `qemu-system-aarch64` and
`qemu-system-riscv64` to the Dockerfile. Bump container version.

**CPU selection in `qemu.go`** — when target arch == host arch and KVM is
available, use `-enable-kvm -cpu host` (fast). When cross-arch, use `-cpu max`
(best emulated feature set). Override the machine definition's `cpu` field at
runtime rather than changing the machine template.

**Direct kernel boot for arm64** — `yoe run` with an arm64 image uses
`-kernel <vmlinuz> -append <cmdline>` instead of firmware boot. The kernel
path comes from the image's build output.

### 6. Machine Template Updates

The existing `qemu-arm64` machine template in `init.go` is correct for both
native and emulated use. The `cpu = "host"` value is overridden at runtime
when KVM is unavailable (see section 5).

No changes needed to machine templates or Starlark definitions.

## What Doesn't Change

- **Starlark evaluation, DAG resolution, source fetch** — all run on host
- **bwrap sandbox config** — unchanged, ARM binaries just work via binfmt
- **Unit definitions** — no changes, they're architecture-agnostic
- **Build scripts** — `./configure && make` works the same under emulation
- **Cache hashing** — arch already part of the hash, x86_64 and arm64 cache
  separately
- **Layer sync** — architecture-independent

## Performance Expectations

QEMU user-mode emulation is ~5-20x slower than native execution, depending on
workload. I/O-bound builds fare better than CPU-bound ones.

- Full system rebuild (rare): expect significant time increase
- Iterating on a few packages: acceptable for development workflow
- Container image build (one-time): ~1-2 minutes for ARM Alpine + toolchain

Future optimization: remote native builder support could be added later for
CI or large rebuilds without changing the user-facing model.

## User Experience

```bash
# New project targeting ARM
$ yoe init my-arm-project --machine qemu-arm64

# First cross-arch build
$ yoe build base-image
Error: binfmt_misc not registered for arm64.
Run 'yoe container binfmt' to enable cross-architecture builds.

# One-time setup
$ yoe container binfmt
This will register QEMU user-mode emulation for foreign architectures...
Proceed? (y/n) y
Done.

# Build works — container auto-built on first use
$ yoe build base-image
[yoe] building container image yoe-ng:11-arm64...
[yoe] container: bwrap ... make -j$(nproc)
...
base-image           [done] a1b2c3d4e5f6

# Run the ARM image in QEMU
$ yoe run base-image
Starting QEMU (host): qemu-system-aarch64 arm64
```
