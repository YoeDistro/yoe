# Metadata File Format

Yoe-NG uses a small set of declarative metadata files to describe how to build
software and assemble system images. All metadata files use
[TOML](https://toml.io/) — it's human-friendly, has a well-defined spec, and has
excellent Go library support.

## Recipes vs. Packages

These are distinct concepts in Yoe-NG:

- **Recipes** — TOML files in the project tree that describe _how to build_
  software. They live in version control and are a development/CI concern.
- **Packages** — `.apk` files that recipes _produce_. They are installable
  artifacts published to a repository and consumed by `apk` during image
  assembly or on-device updates.

The build flow is: **recipe → build → .apk package → repository → image /
device**.

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

### Image Recipe (`recipes/<name>.toml`)

An image is just a recipe with `type = "image"`. Instead of compiling source
code, it assembles a root filesystem from packages and produces a disk image.
Image recipes live in `recipes/` alongside package recipes — they participate in
the same DAG, use the same caching, and are built with `yoe build`.

```toml
[recipe]
name = "base-image"
version = "1.0.0"
type = "image"          # "package" (default) or "image"
description = "Minimal bootable system"

[depends]
# Runtime deps for an image recipe = the packages installed into the rootfs.
# The base system (glibc, busybox, systemd) is implicit unless excluded.
runtime = [
    "openssh",
    "networkmanager",
    "myapp",
    "monitoring-agent",
]

# Image-level configuration
[image]
hostname = "yoe"
timezone = "UTC"
locale = "en_US.UTF-8"
partition-layout = "partitions/default.toml"

# Systemd services to enable
[image.services]
enable = ["sshd", "NetworkManager", "myapp"]
```

### Image Composition and Variants

Image recipes can inherit from other image recipes using the `extends` field,
enabling a base + variant pattern without duplicating package lists.

```toml
# recipes/dev-image.toml — extends base-image with development tools
[recipe]
name = "dev-image"
version = "1.0.0"
type = "image"
description = "Development image with debug tools"
extends = "base-image"   # inherits all deps and config from base-image

[depends]
runtime = [
    "gdb",
    "strace",
    "tcpdump",
    "vim",
]
# Packages can also be excluded from the parent
exclude = [
    "monitoring-agent",   # not needed in dev
]

[image]
hostname = "yoe-dev"      # overrides parent's hostname

[image.services]
enable = ["sshd"]         # merged with parent's services
```

The inheritance chain is resolved during the DAG resolution phase.
`yoe build dev-image` installs everything from `base-image` plus the `dev-image`
additions, minus any exclusions. Deep inheritance is supported but discouraged —
keep it to one level for readability.

**Conditional packages per machine:**

Image recipes can include machine-specific packages using the
`[depends.machine.*]` table:

```toml
[depends]
runtime = ["openssh", "myapp"]

[depends.machine.beaglebone-black]
runtime = ["bbb-dtb-overlay"]

[depends.machine.raspberrypi4]
runtime = ["rpi-firmware", "rpi-dt-overlays"]
```

These are merged with the base dependency list when building for the named
machine.

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
# Content-addressed build cache (local)
path = "/var/cache/yoe-ng/build"

# Remote cache — S3-compatible object storage for sharing builds across
# CI runners and team members. Multiple remotes can be configured;
# they are checked in order after the local cache.
[[cache.remote]]
name = "team"
type = "s3"
bucket = "yoe-cache"
endpoint = "https://minio.internal:9000"  # self-hosted MinIO
region = "us-east-1"
prefix = "v1/"
# credentials via AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY env vars

# [[cache.remote]]
# name = "ci"
# type = "s3"
# bucket = "yoe-ci-cache"
# endpoint = "https://s3.amazonaws.com"
# region = "us-east-1"

# Cache retention — objects not accessed within this period are eligible
# for deletion. Managed by S3 lifecycle policies; this value is advisory
# for `yoe cache gc` on local disk.
[cache.retention]
days = 90

# Package signing for remote cache integrity
[cache.signing]
public-key = "keys/cache.pub"
# private-key is typically provided via YOE_CACHE_SIGNING_KEY env var
# private-key = "keys/cache.key"

[sources]
# Upstream mirrors / proxies for language package managers.
# These reduce external network dependency and speed up builds.
go-proxy = "https://proxy.golang.org"
# cargo-registry = "https://..."
# npm-registry = "https://..."
# pypi-mirror = "https://..."
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
├── recipes/
│   ├── base-image.toml         # type = "image"
│   ├── dev-image.toml          # type = "image", extends base-image
│   ├── openssh.toml            # type = "package" (default)
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
  recipes/*.toml              (all recipe types: package and image)
       │
       ▼
  yoe build                   (resolve DAG, then build in order)
       │
       ├─ type=package ──▶ compile source ──▶ *.apk packages ──▶ repository/
       │
       └─ type=image   ──▶ apk install deps into rootfs
                           ──▶ apply overlays + config
                           ──▶ partition + format
                           ──▶ disk image (.img / .wic)
```

## Label-Based References

Inspired by GN's `//path/to:target` labels, Yoe-NG uses a URI-style scheme for
referencing recipes across repositories. This enables the composability goal of
pulling in recipes from external sources (similar to KAS config composition).

```
# Local recipe (in the current project)
openssh                         # shorthand
recipes/openssh                 # explicit local path

# External recipe (from a GitHub repository)
github.com/yoe/recipes-core//openssh
github.com/vendor/bsp-recipes//kernel-custom

# Pinned to a specific version/commit
github.com/yoe/recipes-core//openssh@v1.2.0
github.com/yoe/recipes-core//openssh@abc123f
```

External recipe references are declared in `distro.toml`:

```toml
[layers]
# Pull in shared recipe collections (similar to Yocto layers or KAS includes)
recipes-core = { url = "github.com/yoe/recipes-core", ref = "v1.0.0" }
bsp-vendor   = { url = "github.com/vendor/bsp-recipes", ref = "main" }
```

When `yoe` resolves the dependency graph, it fetches and caches external recipe
collections, then resolves all references to concrete recipe files. Name
collisions between layers are an error — explicit overrides must be declared.

## Design Notes

- **TOML over YAML/JSON** — avoids YAML's implicit typing pitfalls and JSON's
  lack of comments. TOML is strict enough to prevent ambiguity but readable
  enough to edit by hand.
- **One file per recipe** — each recipe is its own file. This keeps diffs clean
  and makes it easy to add/remove components.
- **Recipes and packages are separate concerns** — recipes are
  version-controlled build instructions; packages are binary artifacts. This
  separation enables building once and deploying many times, sharing packages
  across teams, and on-device incremental updates via `apk`.
- **Shell commands for build steps** — recipe build steps are plain shell
  commands rather than a DSL. This is intentionally simple (like Arch's
  PKGBUILD) and avoids inventing a new abstraction. The build environment is
  controlled and hermetic; the shell commands just describe what to run inside
  it.
- **Unified recipe directory** — system packages, application packages, and
  images all live in `recipes/`. The `type` field determines the output:
  packages produce `.apk` files, images produce disk images. This keeps the
  model simple: one concept (recipe), one directory, one DAG. Images are just
  recipes whose "build step" is assembling a rootfs from their dependencies.
- **apk for image assembly** — image recipes declare their packages as runtime
  dependencies. `yoe build <image>` creates a clean rootfs and runs `apk add` to
  populate it from the repository, exactly like Alpine's image builder. This
  leverages apk's dependency resolution rather than reimplementing it.
