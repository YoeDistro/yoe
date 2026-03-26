# The `yoe` Tool

`yoe` is the single CLI tool that drives all Yoe-NG workflows — building
packages, assembling images, managing source downloads, and flashing devices. It
is a statically-linked Go binary with no runtime dependencies.

## Installation

```sh
# From source
go install github.com/yoe/yoe-ng/cmd/yoe@latest

# Or download a prebuilt binary
curl -L https://github.com/yoe/yoe-ng/releases/latest/download/yoe-$(uname -s)-$(uname -m) -o yoe
chmod +x yoe
```

Since `yoe` is a Go binary, it cross-compiles trivially — build on your x86
workstation, run on an ARM build server.

## Command Overview

```
yoe init            Create a new Yoe-NG project
yoe build           Build packages from recipes
yoe image           Assemble a root filesystem image
yoe flash           Write an image to a device/SD card
yoe run             Run an image in QEMU
yoe repo            Manage the local apk package repository
yoe cache           Manage the build cache (local and remote)
yoe source          Download and manage source archives/repos
yoe config          View and edit project configuration
yoe desc            Describe a recipe, package, or target
yoe refs            Show reverse dependencies
yoe graph           Visualize the dependency DAG
yoe tui             Launch the interactive TUI
yoe clean           Remove build artifacts
```

## Commands

### `yoe init`

Scaffolds a new Yoe-NG project directory with the standard layout.

```sh
yoe init my-project
```

Creates:

```
my-project/
├── distro.toml
├── machines/
├── images/
├── recipes/
├── partitions/
└── overlays/
```

Optionally specify a machine to start with:

```sh
yoe init my-project --machine beaglebone-black
```

### `yoe build`

Builds one or more recipes into `.apk` packages and publishes them to the local
repository.

```sh
# Build a single recipe
yoe build openssh

# Build multiple recipes
yoe build openssh zlib openssl

# Build all recipes
yoe build --all

# Build a recipe and all its dependencies
yoe build --with-deps myapp

# Rebuild even if the cache is fresh
yoe build --force openssh

# Skip remote cache — only check local cache
yoe build --no-remote-cache openssh

# Skip all caches — force build from source
yoe build --no-cache openssh

# Dry run — show what would be built and why
yoe build --dry-run --all
```

**What happens during a build:**

Inspired by Google's GN, `yoe build` uses a **two-phase resolve-then-build**
model. The entire dependency graph is resolved and validated _before_ any build
work starts. This catches missing dependencies, cycles, and configuration errors
up front rather than mid-build.

1. **Resolve dependencies** — read the recipe's `[depends]` table and
   topologically sort the build order. Validate that all referenced recipes
   exist and the graph is acyclic. **If any errors are found, stop here** — no
   partial builds.
2. **Check cache** — compute a content hash of the recipe + source + build
   dependencies. If a cached `.apk` with that hash exists (locally or in a
   remote cache), skip the build.
3. **Fetch source** — download the source archive or clone the git repo (see
   `yoe source` below). Sources are cached in `$YOE_CACHE/sources/`.
4. **Prepare build environment** — set up an isolated build root with only
   declared build dependencies installed via `apk`. This ensures hermetic
   builds.
5. **Execute build steps** — run the recipe's `[build].steps` or
   `[build].command` in the build root. The environment provides:
   - `$PREFIX` — install prefix (typically `/usr`)
   - `$DESTDIR` — staging directory for installed files
   - `$NPROC` — number of available CPU cores
   - `$ARCH` — target architecture
6. **Package** — collect files from `$DESTDIR`, generate `.PKGINFO` from the
   recipe metadata, and create the `.apk` archive.
7. **Publish** — add the `.apk` to the local repository and update the repo
   index.

### `yoe image`

Assembles a root filesystem image from an image definition.

```sh
# Build the default image for the default machine
yoe image

# Specify image and machine
yoe image --image dev --machine raspberrypi4

# Build all images for a specific machine
yoe image --all-images --machine raspberrypi4

# Build a specific image for all machines
yoe image --image base --all-machines

# Build all images for all machines (full matrix)
yoe image --all

# List available image/machine combinations
yoe image --list

# Output a specific format
yoe image --format sdcard    # raw disk image with partitions
yoe image --format rootfs    # tar.gz of the rootfs only
yoe image --format squashfs  # squashfs for read-only roots
```

**What happens during image assembly:**

1. **Read image definition** — parse `images/<name>.toml` for the package list
   and configuration.
2. **Read machine definition** — parse `machines/<name>.toml` for architecture,
   kernel, bootloader, and partition layout.
3. **Create empty rootfs** — set up a temporary directory as the root
   filesystem.
4. **Install packages** — run `apk add --root <rootfs>` with the Yoe-NG
   repository to install all specified packages and their dependencies. apk
   handles dependency resolution.
5. **Apply configuration** — set hostname, timezone, locale, enable systemd
   services per the image definition.
6. **Apply overlays** — copy files from `overlays/` into the rootfs (custom
   configs, static files, etc.).
7. **Install kernel + bootloader** — build (or fetch from cache) the kernel and
   bootloader per the machine definition, install into the rootfs/boot
   partition.
8. **Generate disk image** — partition the output image per the partition layout
   and populate each partition.

### `yoe flash`

Writes a built image to a block device or SD card.

```sh
# Flash to SD card (auto-detects the most recent image)
yoe flash /dev/sdX

# Flash a specific image
yoe flash --image dev --machine beaglebone-black /dev/sdX

# Dry run — show what would happen
yoe flash --dry-run /dev/sdX
```

Safety: `yoe flash` requires explicit confirmation before writing and refuses to
write to mounted devices or devices that look like system disks.

### `yoe run`

Launches a built image in QEMU for development and testing. QEMU runs with KVM
hardware virtualization, so the host and guest architecture must match (e.g.,
x86_64 host runs x86_64 images). For testing other architectures, use native
hardware or native CI runners.

```sh
# Run the most recently built image (auto-detects machine/image)
yoe run

# Run a specific image/machine combination
yoe run --image dev --machine qemu-x86_64

# Forward host port 2222 to guest SSH (port 22)
yoe run --port 2222:22

# Allocate more memory
yoe run --memory 2G

# Run with graphical output (default is serial console)
yoe run --display

# Run headless in the background, SSH only
yoe run --daemon --port 2222:22
```

**What happens:**

1. **Detect architecture** — read the machine definition to determine the target
   architecture (x86_64, aarch64, riscv64).
2. **Select QEMU binary** — map to the correct `qemu-system-*` binary.
3. **Configure machine** — for x86_64, use the `q35` machine type with UEFI
   firmware (OVMF). For aarch64, use `virt` with UEFI (AAVMF). For riscv64, use
   `virt` with OpenSBI.
4. **Enable KVM** — hardware virtualization is always used since host and guest
   architectures match.
5. **Attach image** — use the built disk image as a virtio block device.
6. **Route console** — by default, connect the serial console to the terminal
   (`-nographic`). The guest kernel must have `console=ttyS0` (x86) or
   `console=ttyAMA0` (aarch64) in its command line.
7. **Set up networking** — use QEMU user-mode networking with port forwarding.
   Host-to-guest SSH is available when `--port` is specified.

**QEMU machine definitions:**

Projects can define QEMU-specific machines alongside hardware ones:

```toml
# machines/qemu-x86_64.toml
[machine]
name = "qemu-x86_64"
arch = "x86_64"

[machine.kernel]
recipe = "linux-qemu"
cmdline = "console=ttyS0 root=/dev/vda2 rw"

[machine.qemu]
machine = "q35"
cpu = "host"
memory = "1G"
firmware = "ovmf"
display = "none"
```

When `yoe run` is given a machine with a `[machine.qemu]` section, it uses those
settings directly. When given a hardware machine without a `[machine.qemu]`
section, it falls back to a reasonable default QEMU configuration for the
machine's architecture.

### `yoe repo`

Manages the local apk package repository.

```sh
# List all packages in the repository
yoe repo list

# Show details of a specific package
yoe repo info openssh

# Remove a package from the repository
yoe repo remove openssh-9.5p1-r0

# Push local repository to a remote (S3-compatible)
yoe repo push

# Pull packages from a remote repository
yoe repo pull
```

The local repository lives at the path configured in `distro.toml`
(`[repository].path`). It's a standard apk-compatible repository — you can point
`apk` on a running device at it directly.

### `yoe cache`

Manages the local and remote build caches.

```sh
# Show cache status — local size, remote config, hit rate
yoe cache status

# List cached packages (local)
yoe cache list

# Show what's cached for a specific recipe
yoe cache list openssh

# Push locally-built packages to the remote cache
yoe cache push

# Push specific packages
yoe cache push openssh zlib

# Pull packages from the remote cache into local
yoe cache pull

# Remove local cache entries older than retention period
yoe cache gc

# Remove all local cache entries
yoe cache gc --all

# Verify integrity of cached packages (check hashes and signatures)
yoe cache verify

# Show cache hit/miss statistics for the last build
yoe cache stats
```

**Cache push/pull vs. repo push/pull:** `yoe repo` manages the **apk package
repository** (the repo index that `apk` consumes during image assembly).
`yoe cache` manages the **build cache** (content-addressed build outputs keyed
by input hash). In practice, both store `.apk` files, but the cache is keyed by
build inputs while the repo is indexed by package name/version. Pushing to the
cache shares _build avoidance_ with CI/team. Pushing to the repo shares
_installable packages_ with devices.

### `yoe source`

Manages source downloads. Sources are cached locally to avoid repeated
downloads.

```sh
# Download sources for a recipe
yoe source fetch openssh

# Download sources for all recipes
yoe source fetch --all

# List cached sources
yoe source list

# Verify source integrity (check sha256)
yoe source verify

# Clean stale sources
yoe source clean
```

Sources are stored in `$YOE_CACHE/sources/` with content-addressed naming. For
git sources, bare clones are cached and updated incrementally.

### `yoe config`

View and edit project configuration.

```sh
# Show current configuration
yoe config show

# Set the default machine
yoe config set defaults.machine raspberrypi4

# Set the default image
yoe config set defaults.image dev

# Show resolved configuration for a build
yoe config resolve --machine beaglebone-black --image base
```

### `yoe desc`

Describes a recipe, showing its resolved configuration, dependencies, build
inputs hash, and package output. Inspired by GN's `gn desc`.

```sh
# Show full details of a recipe
yoe desc openssh

# Example output:
#   Recipe:       openssh
#   Version:      9.6p1
#   Source:       https://cdn.openbsd.org/.../openssh-9.6p1.tar.gz
#   Build deps:   zlib, openssl
#   Runtime deps: zlib, openssl
#   Input hash:   a3f8c2...
#   Cached .apk:  yes (openssh-9.6p1-r0.apk)
#   Config:       CFLAGS=-O2 -march=armv8-a (propagated from machine)

# Show only the resolved config for a recipe
yoe desc openssh --config

# Show the build inputs that contribute to the hash
yoe desc openssh --inputs
```

### `yoe refs`

Shows reverse dependencies — what recipes or images depend on a given recipe.
Inspired by GN's `gn refs`.

```sh
# What depends on openssl?
yoe refs openssl

# Example output:
#   Build deps:
#     openssh (build + runtime)
#     curl (build + runtime)
#     python (build)
#   Images:
#     base (via openssh, curl)
#     dev (via openssh, curl, python)

# Show only direct dependents
yoe refs openssl --direct

# Show the full transitive tree
yoe refs openssl --tree
```

This is essential for answering "if I update openssl, what needs to rebuild?"

### `yoe graph`

Visualizes the dependency DAG.

```sh
# Print the dependency graph as text
yoe graph

# Output DOT format for graphviz
yoe graph --format dot | dot -Tpng -o deps.png

# Show graph for a single recipe and its deps
yoe graph openssh

# Show only recipes that need rebuilding
yoe graph --stale
```

### `yoe tui`

Launches an interactive terminal UI for common workflows.

```
┌─ Yoe-NG ─────────────────────────────────────────────┐
│                                                       │
│  Machine: beaglebone-black    Image: base             │
│                                                       │
│  [B] Build packages                                   │
│  [I] Build image                                      │
│  [F] Flash to device                                  │
│  [R] Repository status                                │
│  [M] Select machine                                   │
│  [C] Configuration                                    │
│  [L] Build log                                        │
│                                                       │
│  Packages: 23 built, 2 outdated                       │
│  Last image: 2026-03-19 14:32                         │
│                                                       │
└───────────────────────────────────────────────────────┘
```

The TUI is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
and provides real-time build progress, log streaming, and interactive selection
of machines/images/recipes.

### `yoe clean`

Removes build artifacts.

```sh
# Remove build intermediates (keep cached packages)
yoe clean

# Remove everything (build dirs, packages, sources)
yoe clean --all

# Remove only packages for a specific recipe
yoe clean openssh
```

## Environment Variables

| Variable                | Default           | Description                                     |
| ----------------------- | ----------------- | ----------------------------------------------- |
| `YOE_PROJECT`           | `.` (cwd)         | Path to the Yoe-NG project root                 |
| `YOE_CACHE`             | `~/.cache/yoe-ng` | Cache directory for sources, builds, packages   |
| `YOE_JOBS`              | nproc             | Parallel build jobs                             |
| `YOE_LOG`               | `info`            | Log level (`debug`, `info`, `warn`, `error`)    |
| `YOE_CACHE_SIGNING_KEY` | (none)            | Path to private key for signing cached packages |
| `YOE_NO_REMOTE_CACHE`   | `false`           | Disable remote cache lookups                    |
| `AWS_ACCESS_KEY_ID`     | (none)            | S3 credentials for remote cache                 |
| `AWS_SECRET_ACCESS_KEY` | (none)            | S3 credentials for remote cache                 |
| `AWS_ENDPOINT_URL`      | (none)            | S3 endpoint override (for MinIO / non-AWS)      |

## Dependency Resolution

`yoe` resolves dependencies at two levels:

1. **Build-time** — recipe `[depends].build` entries form a DAG.
   `yoe build --with-deps` topologically sorts this graph and builds in order,
   parallelizing where the DAG allows.

2. **Install-time** — recipe `[depends].runtime` entries are written into the
   `.apk`'s `.PKGINFO`. When `apk add` runs during image assembly, it pulls in
   runtime dependencies automatically.

This means:

- Build dependencies are resolved by `yoe` (it knows the recipe graph).
- Runtime dependencies are resolved by `apk` (it knows the package graph).
- The recipe author declares both; the tools handle the rest.

### Config Propagation

Inspired by GN's `public_configs`, machine-level configuration automatically
propagates through the dependency graph. When you build for a specific machine,
settings like architecture flags, optimization level, and kernel headers path
flow to every recipe without each recipe declaring them:

```
machine (beaglebone-black)
  → arch = "arm64"
  → CFLAGS = "-O2 -march=armv8-a"
  → KERNEL_HEADERS = "/usr/src/linux-6.6/include"
      ↓ propagates to
  recipe (zlib)        → builds with arm64 flags
  recipe (openssl)     → builds with arm64 flags
  recipe (openssh)     → builds with arm64 flags + sees kernel headers
```

Recipes can also declare `public_config` settings that propagate to their
dependents. For example, a `zlib` recipe might export its include path so that
`openssh` (which depends on `zlib`) automatically gets `-I/usr/include` without
the recipe author specifying it.

This is resolved during the graph resolution phase (phase 1) so the full
resolved config for every recipe is known before any build starts. Use
`yoe desc <recipe> --config` to inspect the resolved configuration.

**Design note: recipe-level, not task-level dependencies.** Unlike BitBake,
which models dependencies between individual tasks across recipes (e.g.,
`B:do_configure` depends on `A:do_install`), `yoe` treats each recipe as an
atomic unit — recipe A depends on recipe B means B must be fully built before A
starts. This is a deliberate simplicity trade-off. BitBake's task-level graph
enables fine-grained parallelism (start fetching C while B is still compiling)
and per-task caching (sstate), but it is also the primary source of Yocto's
debugging complexity. Recipe-level dependencies are easier to reason about, and
the parallelism loss is minor since independent recipes still build concurrently
across the DAG. Per-recipe caching via content-addressed `.apk` hashes provides
sufficient granularity for fast incremental rebuilds.

## Caching Strategy

Builds are cached at multiple levels:

1. **Source cache** — downloaded tarballs and git clones in
   `$YOE_CACHE/sources/`. Keyed by URL + hash.
2. **Build cache** — content-addressed by hashing the recipe, source, and all
   build dependency `.apk` hashes. If the combined hash matches, the build is
   skipped and the cached `.apk` is used.
3. **Package repository** — built `.apk` files in the local repo. Once
   published, packages are available for image assembly and on-device updates.
4. **Remote cache** (optional) — push/pull packages to an S3-compatible store so
   CI and team members share build results. See the
   [Caching Architecture](build-environment.md#caching-architecture) section for
   details on S3 configuration, cache signing, and the multi-level fallback
   chain.

Cache invalidation is hash-based, not timestamp-based. Changing a recipe,
updating a source, or rebuilding a dependency all produce a new hash and trigger
a rebuild. Use `yoe build --dry-run` to see what would be rebuilt and why, or
`yoe cache stats` to review hit/miss rates from the last build.

## Example Workflow

```sh
# Start a new project
yoe init my-product --machine beaglebone-black

# Add a recipe for your application
$EDITOR recipes/myapp.toml

# Build everything
yoe build --all

# Assemble the image
yoe image

# Flash to an SD card
yoe flash /dev/sdX

# Later, update just your app
$EDITOR recipes/myapp.toml  # bump version
yoe build myapp
yoe image                    # only myapp's .apk changed, fast rebuild

# Or update the device directly
scp repo/myapp-1.3.0-r0.apk device:/tmp/
ssh device apk add /tmp/myapp-1.3.0-r0.apk
```
