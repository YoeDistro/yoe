# The `yoe` Tool

`yoe` is the single CLI tool that drives all Yoe-NG workflows — building
packages and images from units, managing caches and source downloads, and
flashing devices. It is a statically-linked Go binary with no runtime
dependencies.

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
yoe                 Launch the interactive TUI
yoe init            Create a new Yoe-NG project
yoe build           Build units (packages and images)
yoe dev             Manage source modifications (extract, diff, status)
yoe flash           Write an image to a device/SD card
yoe run             Run an image in QEMU
yoe layer           Manage external layers (fetch, sync, list)
yoe repo            Manage the local apk package repository
yoe cache           Manage the build cache (local and remote)
yoe source          Download and manage source archives/repos
yoe config          View and edit project configuration
yoe desc            Describe a unit, package, or target
yoe refs            Show reverse dependencies
yoe graph           Visualize the dependency DAG
yoe log             Show build log (most recent or specific unit)
yoe diagnose        Launch Claude Code to diagnose a build failure
yoe clean           Remove build artifacts
yoe container       Manage the build container (build, status)
```

All commands except `init`, `version`, and `container` run inside an Alpine
build container automatically. The container is built on first use from
`containers/Dockerfile.build`. See
[Build Environment](build-environment.md#tier-0-bootstrap-layer-automatic-container)
for details.

## Commands

### `yoe init`

Scaffolds a new Yoe-NG project directory with the standard layout.

```sh
yoe init my-project
```

Creates:

```
my-project/
├── PROJECT.star
├── machines/
├── units/
├── classes/
└── overlays/
```

Optionally specify a machine to start with:

```sh
yoe init my-project --machine beaglebone-black
```

### `yoe build`

Builds one or more units. Package units (`unit()`, `autotools()`, etc.) produce
`.apk` packages and publish them to the local repository. Image units
(`image()`) assemble a root filesystem and produce a disk image. The class
function used in the `.star` file determines the behavior — the command is the
same for both.

```sh
# Build a single package unit
yoe build openssh

# Build multiple units
yoe build openssh zlib openssl

# Build an image unit (assembles rootfs, produces disk image)
yoe build base-image

# Build an image for a specific machine
yoe build base-image --machine raspberrypi4

# Build all units (packages and images)
yoe build --all

# Build all image units for all machines (full matrix)
yoe build --all --class image

# Build a unit and all its dependencies
yoe build --with-deps myapp

# Rebuild even if the cache is fresh
yoe build --force openssh

# Skip remote cache — only check local cache
yoe build --no-remote-cache openssh

# Skip all caches — force build from source
yoe build --no-cache openssh

# Dry run — show what would be built and why
yoe build --dry-run --all

# List available image/machine combinations
yoe build --list-targets
```

**What happens during a build:**

Inspired by Google's GN, `yoe build` uses a **two-phase resolve-then-build**
model. The entire dependency graph is resolved and validated _before_ any build
work starts. This catches missing dependencies, cycles, and configuration errors
up front rather than mid-build.

1. **Sync layers** — fetch or update external layers declared in `PROJECT.star`
   (skipped if already up to date). See `yoe layer sync`.
2. **Evaluate Starlark** — load and evaluate all `.star` unit files (including
   those from layers) to produce the set of build targets. Each class function
   call (`unit()`, `autotools()`, `image()`, etc.) registers a target.
3. **Resolve dependencies** — topologically sort the build order from declared
   dependencies. Validate that all referenced units exist and the graph is
   acyclic. **If any errors are found, stop here** — no partial builds.
4. **Check cache** — compute a content hash of the unit + source + build
   dependencies. If a cached `.apk` with that hash exists (locally or in a
   remote cache), skip the build.
5. **Fetch source** — download the source archive or clone the git repo (see
   `yoe source` below). Sources are cached in `$YOE_CACHE/sources/`.
6. **Prepare build environment** — set up an isolated build root with only
   declared build dependencies installed via `apk`. This ensures hermetic
   builds.
7. **Execute build steps** — run the build commands defined by the class
   function in the build root. The environment provides:
   - `$PREFIX` — install prefix (typically `/usr`)
   - `$DESTDIR` — staging directory for installed files
   - `$NPROC` — number of available CPU cores
   - `$ARCH` — target architecture
8. **Package** — collect files from `$DESTDIR`, generate `.PKGINFO` from the
   unit metadata, and create the `.apk` archive.
9. **Publish** — add the `.apk` to the local repository and update the repo
   index.

**For image units** (`image()` class), steps 5-9 are replaced with image
assembly:

1. **Sync layers** — same as above.
2. **Evaluate Starlark** — same as above.
3. **Resolve dependencies** — same as above.
4. **Check cache** — same as above.
5. **Read machine definition** — evaluate `machines/<name>.star` for
   architecture, kernel, bootloader, and partition layout.
6. **Create empty rootfs** — set up a temporary directory.
7. **Install packages** — run `apk add --root <rootfs>` with the Yoe-NG
   repository to install all declared packages. apk handles dependency
   resolution.
8. **Apply configuration** — set hostname, timezone, locale, enable systemd
   services per the image unit's configuration.
9. **Apply overlays** — copy files from `overlays/` into the rootfs.
10. **Install kernel + bootloader** — build (or fetch from cache) the kernel and
    bootloader per the machine definition, install into the rootfs/boot
    partition.
11. **Generate disk image** — partition the output image per the partition
    layout and populate each partition.

Output format can be specified with `--format`:

```sh
yoe build base-image --format sdcard    # raw disk image with partitions
yoe build base-image --format rootfs    # tar.gz of the rootfs only
yoe build base-image --format squashfs  # squashfs for read-only roots
```

### `yoe flash`

Writes a built image to a block device or SD card.

```sh
# Flash to SD card (auto-detects the most recent image build)
yoe flash /dev/sdX

# Flash a specific image unit's output
yoe flash base-image /dev/sdX

# Flash for a specific machine
yoe flash base-image --machine beaglebone-black /dev/sdX

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

# Run a specific image unit
yoe run dev-image --machine qemu-x86_64

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

```python
# machines/qemu-x86_64.star
machine(
    name = "qemu-x86_64",
    arch = "x86_64",
    kernel = kernel(
        unit = "linux-qemu",
        cmdline = "console=ttyS0 root=/dev/vda2 rw",
    ),
    qemu = qemu_config(
        machine = "q35",
        cpu = "host",
        memory = "1G",
        firmware = "ovmf",
        display = "none",
    ),
)
```

When `yoe run` is given a machine with a `qemu` configuration, it uses those
settings directly. When given a hardware machine without `qemu` configuration,
it falls back to a reasonable default QEMU configuration for the machine's
architecture.

### `yoe layer`

Manages external layers — the Git repositories declared in `PROJECT.star` that
provide units, classes, and machine definitions.

```sh
# Fetch/update all layers to the refs declared in PROJECT.star
yoe layer sync

# List all layers with status (fetched, local override, version)
yoe layer list

# Show the full resolved layer tree (including transitive deps from LAYER.star)
yoe layer list --tree

# Show details for a specific layer
yoe layer info @vendor-bsp

# Check for updates — show if upstream has newer tags
yoe layer check-updates
```

**What happens during `yoe layer sync`:**

1. **Read PROJECT.star** — parse the `layers` list.
2. **Read LAYER.star from each layer** — discover transitive dependencies.
3. **Resolve versions** — PROJECT.star versions override transitive deps. If a
   required transitive dep is missing, error with an actionable message.
4. **Fetch/update** — clone or update each layer's Git repo into
   `$YOE_CACHE/layers/`. Checkout the declared ref.
5. **Verify** — confirm that each layer's `LAYER.star` (if present) is valid
   Starlark.

**Layer caching:** Layers are cached in `$YOE_CACHE/layers/` as bare Git
repositories with worktree checkouts at the pinned ref. `yoe layer sync`
performs incremental fetches — only downloading new objects.

**Automatic sync:** `yoe build` automatically runs layer sync if any layer is
missing or if `PROJECT.star` has changed since the last sync. You rarely need to
run `yoe layer sync` manually.

**Local overrides:** Layers with `local = "..."` in PROJECT.star skip fetching
entirely and use the local directory. `yoe layer list` shows these as
`(local: ../path)`.

**Example output of `yoe layer list`:**

```
Layer                              Ref        Status
@units-core                      v1.0.0     up to date
@vendor-bsp-imx8                   v2.1.0     up to date
  └─ @hal-common                   v1.3.0     up to date (transitive)
  └─ @firmware-imx                 v5.4       up to date (transitive)
@my-local-layer                    main       (local: ../my-layer)
```

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

The local repository lives at the path configured in `PROJECT.star`
(`repository(path=...)`). It's a standard apk-compatible repository — you can
point `apk` on a running device at it directly.

### `yoe cache`

Manages the local and remote build caches.

```sh
# Show cache status — local size, remote config, hit rate
yoe cache status

# List cached packages (local)
yoe cache list

# Show what's cached for a specific unit
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
# Download sources for a unit
yoe source fetch openssh

# Download sources for all units
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

Describes a unit, showing its resolved configuration, dependencies, build inputs
hash, and package output. Inspired by GN's `gn desc`.

```sh
# Show full details of a unit
yoe desc openssh

# Example output:
#   Unit:       openssh
#   Version:      9.6p1
#   Source:       https://cdn.openbsd.org/.../openssh-9.6p1.tar.gz
#   Build deps:   zlib, openssl
#   Runtime deps: zlib, openssl
#   Input hash:   a3f8c2...
#   Cached .apk:  yes (openssh-9.6p1-r0.apk)
#   Config:       CFLAGS=-O2 -march=armv8-a (propagated from machine)

# Show only the resolved config for a unit
yoe desc openssh --config

# Show the build inputs that contribute to the hash
yoe desc openssh --inputs
```

### `yoe refs`

Shows reverse dependencies — what units or images depend on a given unit.
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

# Show graph for a single unit and its deps
yoe graph openssh

# Show only units that need rebuilding
yoe graph --stale
```

### `yoe` (no args)

Running `yoe` with no arguments launches an interactive terminal UI showing all
units with their build status.

```
  Yoe-NG  Machine: qemu-x86_64  Image: base-image

  NAME                         CLASS        STATUS
→ base-files                   unit         ● cached
  busybox                      unit         ● cached
  linux                        unit         ▌building...
  musl                         unit         ● waiting
  ncurses                      autotools    ● cached
  openssh                      unit         ● failed
  openssl                      autotools    ● cached
  util-linux                   autotools
  zlib                         autotools    ● cached

  b build  e edit  d diagnose  l log  c clean  / search  q quit
```

#### Status indicators

| Indicator      | Color          | Meaning                     |
| -------------- | -------------- | --------------------------- |
| (none)         | —              | Never built                 |
| `● cached`     | dim/gray       | Built and cached            |
| `● waiting`    | yellow         | Queued, deps building first |
| `▌building...` | flashing green | Actively compiling          |
| `● failed`     | red            | Last build failed           |

When you build a unit, its dependencies appear as "waiting" (yellow), then
transition to "building" (flashing green) as the executor reaches them. Multiple
deps can flash green simultaneously.

#### Key bindings (unit list)

| Key     | Action                                               |
| ------- | ---------------------------------------------------- |
| `b`     | Build selected unit in background                    |
| `e`     | Open unit's `.star` file in `$EDITOR`                |
| `d`     | Launch `claude diagnose` for the unit                |
| `l`     | Open unit's build log in `$EDITOR`                   |
| `a`     | Launch `claude /new-unit`                            |
| `c`     | Clean selected unit's build artifacts (with confirm) |
| `/`     | Search/filter units by name                          |
| `Enter` | Show detail view (build output + log tail)           |
| `B`     | Build all units in background                        |
| `C`     | Clean all build artifacts (with confirm)             |
| `j/k`   | Navigate up/down                                     |
| `q`     | Quit                                                 |

#### Detail view

Pressing Enter on a unit shows a split-pane detail view:

- **BUILD OUTPUT** (top) — executor progress: dependency resolution, cache hits,
  build status for each dep
- **BUILD LOG** (bottom) — tail of the unit's `build.log`, updated in real time
  during a build

| Key   | Action                        |
| ----- | ----------------------------- |
| `Esc` | Return to unit list           |
| `b`   | Build this unit in background |
| `d`   | Launch `claude diagnose`      |
| `l`   | Open build log in `$EDITOR`   |

#### Search

Press `/` to enter search mode. Type to filter — only matching units are shown.
Press Enter to accept the filter, Esc to cancel and show all units.

Builds call `build.BuildUnits()` directly (in-process, no subprocess). The
executor sends events to the TUI as each unit starts and finishes building.

The TUI is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

### `yoe log`

Shows a build log. With no arguments, shows the most recently modified build
log. Specify a unit name to view that unit's log.

```
yoe log                  # show most recent build log
yoe log openssl          # show openssl build log
yoe log openssl -e       # open openssl build log in $EDITOR
```

The `-e` / `--edit` flag opens the log in your editor (defaults to `vi`).

### `yoe diagnose`

Launches Claude Code to diagnose a build failure. With no arguments, diagnoses
the most recent build failure. Specify a unit name to diagnose that unit.

```
yoe diagnose             # diagnose most recent failure
yoe diagnose util-linux  # diagnose util-linux build failure
```

Requires `claude` to be in your PATH. Claude Code reads the build log and
iteratively identifies root causes, applies fixes, and rebuilds until the unit
succeeds.

### Custom Commands

Projects can define custom commands in `commands/*.star` that become first-class
`yoe` subcommands. This is similar to Zephyr's `west` extensions but uses
Starlark instead of Python classes.

```python
# commands/deploy.star
command(
    name = "deploy",
    description = "Deploy image to target device via SSH",
    args = [
        arg("target", required=True, help="Target device hostname/IP"),
        arg("--image", default="base-image", help="Image to deploy"),
        arg("--reboot", type="bool", help="Reboot after install"),
    ],
)

def run(ctx):
    img = ctx.args.image
    target = ctx.args.target
    ctx.log("Deploying", img, "to", target)
    ctx.shell("scp", "build/output/" + img + ".img", "root@" + target + ":/tmp/update.img")
    ctx.shell("ssh", "root@" + target, "rauc", "install", "/tmp/update.img")
    if ctx.args.reboot == "true":
        ctx.shell("ssh", "root@" + target, "reboot")
```

Usage:

```sh
yoe deploy 192.168.1.100 --image production-image --reboot
```

Custom commands show up alongside built-in commands. If `yoe` doesn't recognize
a command, it checks `commands/*.star` before printing "unknown command".

**The context object** provides:

| Method                | Description                              |
| --------------------- | ---------------------------------------- |
| `ctx.args.<name>`     | Parsed command-line arguments            |
| `ctx.shell(cmd, ...)` | Execute a shell command (returns output) |
| `ctx.log(msg, ...)`   | Print a message                          |
| `ctx.project_root`    | Path to the project root                 |

**Commands from layers:**

Vendor BSP layers can ship custom commands (e.g., `flash-emmc`, `enter-dfu`)
that become available when the layer is added to the project.

**Key difference from unit evaluation:** Unit `.star` files are sandboxed — no
I/O, deterministic. Command `.star` files have full I/O access via `ctx.shell()`
because they are actions, not build definitions.

### `yoe dev`

Work with unit source code directly. Every unit's build directory is a git repo
— upstream source is committed with an `upstream` tag, and existing patches are
applied as commits on top. Local edits are just git commits.

There is no "dev mode" to enter or exit. If the build directory has commits
beyond `upstream`, `yoe build` uses them directly instead of re-fetching source.

```sh
# After building, edit source in place
yoe build openssh
cd build/openssh/src
vim auth.c
git commit -am "fix auth timeout handling"

# Rebuild uses your local commits
yoe build openssh

# See what you've changed
yoe dev diff openssh

# Extract commits as patch files
yoe dev extract openssh
# Writes patches/openssh/0001-fix-auth-timeout-handling.patch
# Prints updated patches list for your unit

# Check which units have local modifications
yoe dev status
```

**Subcommands:**

| Subcommand               | Description                                                                                   |
| ------------------------ | --------------------------------------------------------------------------------------------- |
| `yoe dev extract <unit>` | Run `git format-patch upstream..HEAD`, write to `patches/<unit>/`, print updated patches list |
| `yoe dev diff <unit>`    | Show `git log upstream..HEAD` — your local commits                                            |
| `yoe dev status`         | List all units with commits beyond upstream                                                   |

**Rebasing on upstream updates:**

```sh
# Update unit version
$EDITOR units/openssh.star   # bump version to 9.7p1

# Rebuild fetches new source, applies patches via rebase
yoe build openssh

# If patches conflict, resolve in the git repo
cd build/openssh/src
git rebase --continue
yoe dev extract openssh         # re-extract clean patches
```

**Why this is simpler than Yocto's devtool:**

- No separate workspace — the build directory is the workspace
- No mode to enter/exit — local commits are automatically detected
- No state files — git is the only state
- Extracting patches is `git format-patch` — a command developers already know
- Each patch = one git commit, so the patch series is the git log

### `yoe clean`

Removes build artifacts.

```sh
# Remove build intermediates (keep cached packages)
yoe clean

# Remove everything (build dirs, packages, sources)
yoe clean --all

# Remove only packages for a specific unit
yoe clean openssh
```

## Environment Variables

| Variable                | Default   | Description                                     |
| ----------------------- | --------- | ----------------------------------------------- |
| `YOE_PROJECT`           | `.` (cwd) | Path to the Yoe-NG project root                 |
| `YOE_CACHE`             | `cache/`  | Cache directory for sources, builds, packages   |
| `YOE_JOBS`              | nproc     | Parallel build jobs                             |
| `YOE_LOG`               | `info`    | Log level (`debug`, `info`, `warn`, `error`)    |
| `YOE_CACHE_SIGNING_KEY` | (none)    | Path to private key for signing cached packages |
| `YOE_NO_REMOTE_CACHE`   | `false`   | Disable remote cache lookups                    |
| `AWS_ACCESS_KEY_ID`     | (none)    | S3 credentials for remote cache                 |
| `AWS_SECRET_ACCESS_KEY` | (none)    | S3 credentials for remote cache                 |
| `AWS_ENDPOINT_URL`      | (none)    | S3 endpoint override (for MinIO / non-AWS)      |

## Dependency Resolution

`yoe` resolves dependencies at two levels:

1. **Build-time** — unit `deps` entries form a DAG. `yoe build --with-deps`
   topologically sorts this graph and builds in order, parallelizing where the
   DAG allows.

2. **Install-time** — unit `runtime_deps` entries are written into the `.apk`'s
   `.PKGINFO`. When `apk add` runs during image assembly, it pulls in runtime
   dependencies automatically.

This means:

- Build dependencies are resolved by `yoe` (it knows the unit graph).
- Runtime dependencies are resolved by `apk` (it knows the package graph).
- The unit author declares both; the tools handle the rest.

### Config Propagation

Inspired by GN's `public_configs`, machine-level configuration automatically
propagates through the dependency graph. When you build for a specific machine,
settings like architecture flags, optimization level, and kernel headers path
flow to every unit without each unit declaring them:

```
machine (beaglebone-black)
  → arch = "arm64"
  → CFLAGS = "-O2 -march=armv8-a"
  → KERNEL_HEADERS = "/usr/src/linux-6.6/include"
      ↓ propagates to
  unit (zlib)        → builds with arm64 flags
  unit (openssl)     → builds with arm64 flags
  unit (openssh)     → builds with arm64 flags + sees kernel headers
```

Units can also declare `public_config` settings that propagate to their
dependents. For example, a `zlib` unit might export its include path so that
`openssh` (which depends on `zlib`) automatically gets `-I/usr/include` without
the unit author specifying it.

This is resolved during the graph resolution phase (phase 1) so the full
resolved config for every unit is known before any build starts. Use
`yoe desc <unit> --config` to inspect the resolved configuration.

**Design note: unit-level, not task-level dependencies.** Unlike BitBake, which
models dependencies between individual tasks across units (e.g.,
`B:do_configure` depends on `A:do_install`), `yoe` treats each unit as an atomic
unit — unit A depends on unit B means B must be fully built before A starts.
This is a deliberate simplicity trade-off. BitBake's task-level graph enables
fine-grained parallelism (start fetching C while B is still compiling) and
per-task caching (sstate), but it is also the primary source of Yocto's
debugging complexity. Unit-level dependencies are easier to reason about, and
the parallelism loss is minor since independent units still build concurrently
across the DAG. Per-unit caching via content-addressed `.apk` hashes provides
sufficient granularity for fast incremental rebuilds.

## Caching Strategy

Builds are cached at multiple levels:

1. **Source cache** — downloaded tarballs and git clones in
   `$YOE_CACHE/sources/`. Keyed by URL + hash.
2. **Build cache** — content-addressed by hashing the unit, source, and all
   build dependency `.apk` hashes. If the combined hash matches, the build is
   skipped and the cached `.apk` is used.
3. **Package repository** — built `.apk` files in the local repo. Once
   published, packages are available for image assembly and on-device updates.
4. **Remote cache** (optional) — push/pull packages to an S3-compatible store so
   CI and team members share build results. See the
   [Caching Architecture](build-environment.md#caching-architecture) section for
   details on S3 configuration, cache signing, and the multi-level fallback
   chain.

Cache invalidation is hash-based, not timestamp-based. Changing a unit, updating
a source, or rebuilding a dependency all produce a new hash and trigger a
rebuild. Use `yoe build --dry-run` to see what would be rebuilt and why, or
`yoe cache stats` to review hit/miss rates from the last build.

## Example Workflow

```sh
# Start a new project
yoe init my-product --machine beaglebone-black

# Add a unit for your application
$EDITOR units/myapp.star

# Build everything (packages and images)
yoe build --all

# Flash to an SD card
yoe flash base-image /dev/sdX

# Later, update just your app and rebuild the image
$EDITOR units/myapp.star  # bump version
yoe build myapp
yoe build base-image         # only myapp's .apk changed, fast rebuild

# Or update the device directly
scp repo/myapp-1.3.0-r0.apk device:/tmp/
ssh device apk add /tmp/myapp-1.3.0-r0.apk
```
