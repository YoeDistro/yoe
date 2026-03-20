# Metadata File Format

Yoe-NG uses a small set of declarative metadata files to describe how to build
software and assemble system images. All metadata files use
[TOML](https://toml.io/) — it's human-friendly, has a well-defined spec, and
has excellent Go library support.

## Recipes vs. Packages

These are distinct concepts in Yoe-NG:

- **Recipes** — TOML files in the project tree that describe *how to build*
  software. They live in version control and are a development/CI concern.
- **Packages** — `.apk` files that recipes *produce*. They are installable
  artifacts published to a repository and consumed by `apk` during image
  assembly or on-device updates.

The build flow is: **recipe → build → .apk package → repository → image / device**.

Recipes are inputs to the build system. Packages are outputs. A developer edits
recipes; a device only ever sees packages.

## Why TOML

- Readable and writable by humans without tooling.
- Strict spec — no YAML-style gotchas (Norway problem, implicit type coercion).
- Comments are first-class — metadata files are self-documenting.
- Native Go support via `BurntSushi/toml` or `pelletier/go-toml`.
- Simpler than JSON5/JSONC for configuration use cases.

## Recipe Types

### Machine Definition (`machines/<name>.toml`)

Describes a target board or platform.

```toml
[machine]
name = "beaglebone-black"
arch = "arm"            # arm, arm64, riscv64, x86_64
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
partition-layout = "partitions/bbb.toml"  # reference to partition layout file
```

### Image Definition (`images/<name>.toml`)

Defines what packages go into a root filesystem image. Image assembly uses `apk`
to install packages from the Yoe-NG repository into a clean rootfs.

```toml
[image]
name = "base"
description = "Minimal bootable system"

[packages]
# Package names as they appear in the apk repository.
# The base system (glibc, busybox, systemd) is implicit unless excluded.
include = [
    "openssh",
    "networkmanager",
    "myapp",
    "monitoring-agent",
]

# Image-level configuration
[config]
hostname = "yoe"
timezone = "UTC"
locale = "en_US.UTF-8"

# Systemd services to enable
[services]
enable = ["sshd", "NetworkManager", "myapp"]
```

### Package Recipe (`recipes/<name>.toml`)

Describes how to build a system-level package (C/C++ libraries, system daemons,
etc.) and produce an `.apk`. This is analogous to an Arch PKGBUILD or an Alpine
APKBUILD.

```toml
[recipe]
name = "openssh"
version = "9.6p1"
description = "OpenSSH client and server"
license = "BSD"

[source]
url = "https://cdn.openbsd.org/pub/OpenBSD/OpenSSH/portable/openssh-9.6p1.tar.gz"
sha256 = "..."

[depends]
# Build-time dependencies (must be built/installed before this recipe)
build = ["zlib", "openssl"]
# Runtime dependencies (recorded in the .apk, pulled in by apk on install)
runtime = ["zlib", "openssl"]

[build]
# Build steps — plain shell commands run in the source directory.
# The environment provides $PREFIX, $DESTDIR, $NPROC, and standard flags.
steps = [
    "./configure --prefix=$PREFIX --sysconfdir=/etc/ssh",
    "make -j$NPROC",
    "make DESTDIR=$DESTDIR install",
]

[package]
# Metadata that ends up in the .apk's .PKGINFO
units = ["sshd.service"]
conffiles = ["/etc/ssh/sshd_config"]
```

### Application Recipe (`recipes/<name>.toml`)

Describes an application built with a language-native build system. The recipe
delegates the build to the language toolchain and packages the result as an
`.apk`.

```toml
[recipe]
name = "myapp"
version = "1.2.3"
description = "Edge data collection service"
license = "Apache-2.0"
language = "go"         # go, rust, zig, python, javascript

[source]
repo = "https://github.com/example/myapp.git"
tag = "v1.2.3"
# Or track a branch:
# branch = "main"

[depends]
build = []
runtime = []

[build]
# Language-specific build command (run in the source checkout)
command = "go build -o $DESTDIR/usr/bin/myapp ./cmd/myapp"

[package]
units = ["myapp.service"]
conffiles = ["/etc/myapp/config.toml"]
environment = { DATA_DIR = "/var/lib/myapp" }
```

Note: both system packages and application packages use the same `[recipe]`
header and produce `.apk` files. The distinction is in how they build (shell
steps vs. language toolchain) and what they depend on. They share a single
`recipes/` directory.

### Distro Configuration (`distro.toml`)

Top-level configuration that ties everything together.

```toml
[distro]
name = "yoe"
version = "0.1.0"
description = "Yoe-NG embedded Linux distribution"

[defaults]
machine = "qemu-arm64"
image = "base"

[repository]
# Local package repository (populated by builds)
path = "/var/cache/yoe-ng/repo"
# Optional remote repository (S3-compatible, for sharing built packages)
# remote = "s3://yoe-ng-repo/packages"

[cache]
# Content-addressed build cache
path = "/var/cache/yoe-ng/build"

[sources]
# Upstream mirrors / proxies for language package managers
go-proxy = "https://proxy.golang.org"
# cargo-registry = "https://..."
# npm-registry = "https://..."
```

### Partition Layout (`partitions/<name>.toml`)

Defines disk partition layout for image generation.

```toml
[disk]
type = "gpt"            # gpt or mbr

[[partition]]
label = "boot"
type = "vfat"
size = "64M"
contents = ["MLO", "u-boot.img", "zImage", "*.dtb"]

[[partition]]
label = "rootfs"
type = "ext4"
size = "fill"           # use remaining space
root = true
```

## Directory Structure

A typical Yoe-NG project layout:

```
my-project/
├── distro.toml
├── machines/
│   ├── beaglebone-black.toml
│   ├── raspberrypi4.toml
│   └── qemu-arm64.toml
├── images/
│   ├── base.toml
│   └── dev.toml
├── recipes/
│   ├── openssh.toml
│   ├── zlib.toml
│   ├── openssl.toml
│   ├── myapp.toml
│   └── monitoring-agent.toml
├── partitions/
│   ├── bbb.toml
│   └── rpi.toml
└── overlays/
    └── custom-configs/     # files copied directly into rootfs
        └── etc/
            └── myapp/
                └── config.toml
```

## Build Flow

```
  recipes/*.toml          (build-time: how to build)
       │
       ▼
  yoe-ng build            (invoke build steps / language toolchains)
       │
       ▼
  *.apk packages          (installable artifacts)
       │
       ▼
  repository/             (apk-compatible package repo)
       │
       ▼
  yoe-ng image            (apk install into rootfs + partition layout)
       │
       ▼
  disk image              (flashable .img / .wic)
```

## Design Notes

- **TOML over YAML/JSON** — avoids YAML's implicit typing pitfalls and JSON's
  lack of comments. TOML is strict enough to prevent ambiguity but readable
  enough to edit by hand.
- **One file per recipe** — each recipe is its own file. This keeps diffs clean
  and makes it easy to add/remove components.
- **Recipes and packages are separate concerns** — recipes are version-controlled
  build instructions; packages are binary artifacts. This separation enables
  building once and deploying many times, sharing packages across teams, and
  on-device incremental updates via `apk`.
- **Shell commands for build steps** — recipe build steps are plain shell
  commands rather than a DSL. This is intentionally simple (like Arch's
  PKGBUILD) and avoids inventing a new abstraction. The build environment is
  controlled and hermetic; the shell commands just describe what to run inside
  it.
- **Unified recipe directory** — system packages and application packages both
  live in `recipes/`. The output is always an `.apk` regardless of whether the
  build used `make` or `cargo`. This keeps the model simple: recipe in,
  package out.
- **apk for image assembly** — image definitions are just package lists. The
  `yoe-ng image` command creates a clean rootfs and runs `apk add` to populate
  it from the repository, exactly like Alpine's image builder. This leverages
  apk's dependency resolution rather than reimplementing it.
