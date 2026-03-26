# Build Environment

How Yoe-NG manages host tools, build isolation, and the bootstrap process.

## Architecture

Yoe-NG uses a layered build environment with three tiers:

```
┌─────────────────────────────────────────────────────┐
│  Tier 0: Host / Alpine Container                    │
│  Provides: apk-tools, bubblewrap, yoe (Go binary)  │
│  libc: doesn't matter (musl or glibc)              │
├─────────────────────────────────────────────────────┤
│  Tier 1: Yoe-NG Build Root (chroot/bwrap)           │
│  Populated by: apk from Yoe-NG's package repo       │
│  Provides: glibc, gcc, make, cmake, language SDKs   │
│  libc: glibc (Yoe-NG's own packages)               │
├─────────────────────────────────────────────────────┤
│  Tier 2: Per-Recipe Build Environment               │
│  Populated by: apk with only declared build deps    │
│  Isolated via bubblewrap                            │
│  Produces: .apk packages                            │
└─────────────────────────────────────────────────────┘
```

### Tier 0: Bootstrap Layer

The outermost layer provides just enough to run `yoe` and `apk`. This can be:

- An **Alpine container** — the recommended starting point. Alpine is ~5MB,
  includes `apk`, and has official multi-arch images for x86_64, aarch64, and
  riscv64 (community). Pull and go.
- A **host Linux system** with `apk-tools` and `bubblewrap` installed — for
  developers who prefer not to use Docker.
- **Any environment that can run a static Go binary** — since `yoe` is
  statically linked, it runs anywhere. It only needs `apk-tools` and `bwrap` as
  external dependencies.

The libc used by Tier 0 is irrelevant. `yoe` is a static Go binary. `apk-tools`
can be statically linked. Neither cares whether the outer environment is musl
(Alpine) or glibc. All real build work happens in Tier 1/2.

### Tier 1: Yoe-NG Build Root

A glibc-based environment populated from Yoe-NG's own package repository. This
is where the actual compilers, toolchains, and language SDKs live.

```sh
# yoe creates this automatically during build
apk --root /var/yoe/buildroot \
    --repo https://repo.yoe-ng.org/packages \
    add glibc gcc g++ make cmake go rust
```

This build root is:

- **glibc-based** — Yoe-NG's own packages, not Alpine's.
- **Persistent** — created once, updated as needed. Not torn down between
  builds.
- **Architecture-native** — on an ARM64 machine, it's an ARM64 build root. No
  cross-compilation.
- **Managed by apk** — adding or updating a host tool is just
  `apk add --root ... <tool>`.

### Tier 2: Per-Recipe Isolation

Each recipe builds in an isolated environment with only its declared
dependencies. This ensures hermetic builds — a recipe cannot accidentally depend
on a tool it didn't declare.

```sh
# yoe creates a minimal environment for each recipe build
bwrap \
    --ro-bind /var/yoe/buildroot / \
    --bind /tmp/build/$RECIPE /build \
    --bind /tmp/destdir/$RECIPE /destdir \
    --dev /dev \
    --proc /proc \
    -- sh -c "$BUILD_STEPS"
```

Bubblewrap provides:

- **Unprivileged isolation** — no root or Docker daemon required.
- **Read-only base** — the build root is mounted read-only; recipes can't modify
  host tools.
- **Minimal overhead** — bubblewrap is a thin namespace wrapper, not a full
  container runtime. Build performance is near-native.
- **Declared dependencies only** — the build environment is assembled from only
  the packages listed in the recipe's `[depends].build`.

## Why Not Docker for Builds?

Docker is used for Tier 0 (the bootstrap) but not for Tier 1/2 (the actual
builds). This is deliberate:

|                       | Docker                     | bubblewrap + apk          |
| --------------------- | -------------------------- | ------------------------- |
| Requires root/daemon  | Yes (dockerd)              | No (unprivileged)         |
| Startup overhead      | ~200ms per container       | ~1ms per sandbox          |
| Layering granularity  | Image layers (coarse)      | apk packages (fine)       |
| Dependency management | Dockerfile (imperative)    | apk (declarative)         |
| Nested builds         | Docker-in-Docker (fragile) | Just works                |
| CI integration        | Needs DinD or socket mount | Runs inside any container |

Docker is great for the "zero setup" onboarding story: `docker run yoe/builder`
and you have a working environment. But for the build system itself, bubblewrap

- apk is simpler, faster, and more granular.

## Bootstrap Process

There is a chicken-and-egg problem: Yoe-NG needs glibc, gcc, and other base
packages in its repository before it can build anything inside a Yoe-NG chroot.
This is solved with a staged bootstrap, the same approach used by Alpine, Arch,
Gentoo, and every other self-hosting distribution.

### Stage 0: Cross-Pollination

Build the initial base packages using an existing distribution's toolchain.
Alpine's gcc (or any host gcc) builds the first generation of Yoe-NG packages.

```sh
# Inside Alpine (or any Linux with gcc)
yoe bootstrap stage0

# This builds:
#   glibc         → glibc-2.39-r0.apk
#   binutils      → binutils-2.42-r0.apk
#   gcc           → gcc-14.1-r0.apk
#   linux-headers → linux-headers-6.6-r0.apk
#   busybox       → busybox-1.36-r0.apk
#   apk-tools     → apk-tools-2.14-r0.apk
#   bubblewrap    → bubblewrap-0.9-r0.apk
```

These packages are built with Alpine's musl-based gcc targeting glibc. The
output is a minimal set of `.apk` files — enough to create a self-hosting Yoe-NG
build root.

### Stage 1: Self-Hosting

Rebuild the base packages using the Stage 0 packages. Now the Yoe-NG build root
is building itself.

```sh
yoe bootstrap stage1

# Creates a Yoe-NG build root from Stage 0 packages, then rebuilds:
#   glibc, gcc, binutils, etc. — now built with Yoe-NG's own gcc + glibc
```

After Stage 1, the bootstrap is complete. All packages in the repository were
built by Yoe-NG's own toolchain. The Alpine dependency is gone.

### Stage 2: Normal Operation

From this point on, all builds use the Yoe-NG build root. New recipes build
inside Tier 2 isolated environments. The bootstrap is a one-time cost per
architecture.

```sh
# Normal development — no bootstrap needed
yoe build myapp
yoe image
yoe flash /dev/sdX
```

### Pre-Built Bootstrap

For most users, the bootstrap is not needed at all. Yoe-NG publishes pre-built
base packages for each supported architecture:

- `x86_64` — built in CI
- `aarch64` — built on ARM64 CI runners
- `riscv64` — built on RISC-V hardware or QEMU

A new project pulls these from the Yoe-NG package repository and starts building
immediately. The bootstrap process is only needed by:

- Yoe-NG distribution developers maintaining the base packages.
- Users who need to verify the full build chain for compliance/traceability.
- Users targeting a new architecture.

## Pseudo-Root via User Namespaces

Image assembly requires root-like operations — setting file ownership to
root:root, creating device nodes, setting setuid bits. Traditionally this is
solved with `fakeroot` or Yocto's `pseudo`, both of which use LD_PRELOAD to
intercept libc calls. These approaches are fragile:

| Approach            | Mechanism           | Breaks with Go/static bins | Database corruption | Parallel safety |
| ------------------- | ------------------- | -------------------------- | ------------------- | --------------- |
| fakeroot            | LD_PRELOAD          | Yes                        | N/A                 | Fragile         |
| pseudo (Yocto)      | LD_PRELOAD + SQLite | Yes                        | Yes (known issue)   | Better          |
| **User namespaces** | **Kernel**          | **No**                     | **N/A (stateless)** | **Yes**         |

Yoe-NG uses **user namespaces** (via bubblewrap, already in the stack for build
isolation) for all operations that need pseudo-root access. Inside a user
namespace, the process sees itself as uid 0 and can perform all root-like
filesystem operations — no LD_PRELOAD, no daemon, no database.

### How `yoe image` Uses This

```sh
# Image assembly inside a user namespace
bwrap --unshare-user --uid 0 --gid 0 \
    --bind /tmp/rootfs /rootfs \
    --bind /tmp/output /output \
    --dev /dev \
    --proc /proc \
    -- sh -c '
        # Install packages — apk sets ownership to root:root
        apk --root /rootfs add glibc busybox systemd openssh myapp

        # Create device nodes
        mknod /rootfs/dev/null c 1 3
        mknod /rootfs/dev/console c 5 1

        # Set permissions
        chmod 4755 /rootfs/usr/bin/su

        # Generate filesystem image with correct ownership
        mksquashfs /rootfs /output/rootfs.squashfs
    '
```

Because this is kernel-native:

- **Works with everything** — Go binaries, Rust binaries, statically linked
  tools, anything. No libc interception needed.
- **Stateless** — no SQLite database to corrupt, no daemon to crash. The kernel
  tracks ownership within the namespace.
- **Fast** — namespace creation is ~1ms. No overhead per filesystem operation.
- **Already available** — bubblewrap is already a Tier 0 dependency for build
  isolation. No new tools needed.

### Disk Image Partitioning

For the final step of creating a partitioned disk image (GPT/MBR with boot and
rootfs partitions), `yoe` can use **systemd-repart** as a complementary tool.
Since Yoe-NG already uses systemd, `systemd-repart` is a natural fit:

- Declarative partition definitions (aligns with the TOML partition layout
  files).
- Handles GPT, MBR, filesystem creation, and image sizing.
- Runs unprivileged with user namespaces.
- Maintained by the systemd project.

The combination is: **bubblewrap** for rootfs population (installing packages,
setting ownership, creating device nodes) and **systemd-repart** for disk image
assembly (partitioning, filesystem creation, writing the final `.img`).

## Build Environment Lifecycle

```
First time setup:
  docker pull alpine → install yoe, apk-tools, bwrap
  yoe init my-project
  yoe repo pull              ← fetch pre-built base packages
  yoe build --all            ← build project recipes in Yoe-NG chroot

Day-to-day development:
  $EDITOR recipes/myapp.toml
  yoe build myapp            ← builds in isolated bwrap sandbox
  yoe image                  ← assembles rootfs with apk
  yoe flash /dev/sdX

Adding a host tool:
  $EDITOR recipes/cmake.toml ← write a recipe for the tool
  yoe build cmake            ← produces cmake.apk
  (cmake is now available as a build dependency for other recipes)

Updating the base toolchain:
  yoe build --force gcc      ← rebuild gcc recipe
  yoe build --all            ← rebuild everything against new gcc
```

## Caching Architecture

Yoe-NG's content-addressed build cache is designed around a multi-level fallback
chain. Each level is checked in order; the first hit wins.

### Cache Levels

```
┌──────────────────────────────────────────────────┐
│  Level 1: Local Disk Cache                       │
│  $YOE_CACHE/build/                               │
│  Fastest — no network. Populated by local builds │
├──────────────────────────────────────────────────┤
│  Level 2: LAN / Self-Hosted Cache (optional)     │
│  MinIO or S3-compatible on local network         │
│  ~1ms latency. Shared across team workstations   │
├──────────────────────────────────────────────────┤
│  Level 3: Remote Cache (optional)                │
│  AWS S3, GCS, R2, Backblaze B2, etc.            │
│  Shared across CI runners and distributed teams  │
└──────────────────────────────────────────────────┘
```

### Why S3-Compatible Storage

Content-addressed packages are **immutable, write-once blobs** keyed by their
input hash. This maps directly to S3's key-value object model:

- **No coordination** — multiple CI runners push/pull concurrently without
  locking. Two builders producing the same hash write the same content; last
  writer wins harmlessly.
- **Widely available** — AWS S3, MinIO (self-hosted), GCS, Cloudflare R2, and
  Backblaze B2 all speak the same API. No vendor lock-in.
- **Built-in lifecycle management** — S3 lifecycle policies handle cache
  eviction (e.g., delete objects not accessed in 90 days). No custom garbage
  collection needed.
- **Right granularity** — S3 GET latency (~50-100ms) is negligible at
  package-level granularity. A cache hit that avoids a 5-minute GCC build is
  worth 100ms of network overhead.

Self-hosted MinIO is the recommended starting point for teams that want shared
caching without cloud dependency. It runs as a single binary, supports the full
S3 API, and works in air-gapped environments.

### Cache Key Computation

The cache key for a recipe is a cryptographic hash of:

- The recipe TOML file contents
- The source archive/commit hash
- The `.apk` hashes of all build dependencies (transitive)
- The machine architecture and propagated build flags

This means any change to a recipe, its source, or any of its dependencies
produces a new cache key. Cache invalidation is automatic — there are no stale
entries, only unused ones.

### Language Package Manager Caches

Language-native package managers (Go modules, Cargo crates, npm packages, pip
wheels) have their own download caches. Yoe-NG shares these across builds:

- **Go** — `GOMODCACHE` is set to a shared directory; the Go module proxy
  (`GOPROXY`) can point to a local Athens instance or the public
  `proxy.golang.org`.
- **Rust** — `CARGO_HOME` is shared; a local
  [Panamax](https://github.com/panamax-rs/panamax) mirror can serve as a
  registry cache.
- **Node.js** — `npm_config_cache` is shared; a local Verdaccio instance can
  proxy the npm registry.
- **Python** — `PIP_CACHE_DIR` is shared; a local devpi instance can proxy PyPI.

These caches are **not** content-addressed by Yoe-NG — they are managed by the
language toolchains themselves. Yoe-NG ensures the cache directories persist
across builds and are shared across recipes that use the same language.

### Cache Signing and Verification

Packages pushed to a remote cache are signed with a project-level key. When
pulling from a remote cache, `yoe` verifies the signature before using the
cached package. This prevents cache poisoning — a compromised cache server
cannot inject malicious packages.

The signing key is configured in `distro.toml` (`[cache.signing]`). For CI, the
private key is provided via environment variable; workstations can use a
read-only public key for verification only.

## Multi-Target Builds

A single Yoe-NG project can define multiple machines and multiple images,
building any combination from the same source tree. This is similar to Yocto's
multi-machine/multi-image capability but with simpler mechanics.

### How It Works

Machines and images are independent axes. A machine defines _what hardware_ to
build for (architecture, kernel, bootloader, partition layout). An image defines
_what software_ to include (package list, services, configuration). Any image
can be built for any compatible machine.

```
machines/                    images/
├── beaglebone-black.toml    ├── base.toml
├── raspberrypi4.toml        ├── dev.toml
└── qemu-arm64.toml          └── production.toml

Build matrix:
  yoe image --machine beaglebone-black --image base
  yoe image --machine beaglebone-black --image dev
  yoe image --machine raspberrypi4 --image production
  yoe image --all   ← builds all valid combinations
```

### Package Sharing Across Targets

Because recipes produce architecture-specific `.apk` packages that live in a
shared repository, packages built for one machine are reused by any other
machine with the same architecture. Building `openssh` for the BeagleBone also
satisfies the Raspberry Pi — both are `aarch64` and produce identical packages
(same recipe, same source, same arch flags → same cache key).

This means a multi-machine project does **not** rebuild the world for each
board. Only machine-specific packages (kernel, bootloader, device trees) are
built per-machine. Everything else comes from cache.

### Build Output Organization

Build outputs are organized by machine and image:

```
build/output/
├── beaglebone-black/
│   ├── base/
│   │   └── base-beaglebone-black.img
│   └── dev/
│       └── dev-beaglebone-black.img
├── raspberrypi4/
│   └── production/
│       └── production-raspberrypi4.img
└── repo/
    └── aarch64/           ← shared package repo for all aarch64 machines
        ├── openssh-9.6p1-r0.apk
        ├── myapp-1.2.3-r0.apk
        └── ...
```

### Architecture Isolation

When a project targets multiple architectures (e.g., `aarch64` and `x86_64`),
each architecture gets its own Tier 1 build root and package repository.
Packages from different architectures never mix. The build roots are:

```
/var/yoe/buildroot/aarch64/    ← aarch64 compilers, libraries
/var/yoe/buildroot/x86_64/     ← x86_64 compilers, libraries
```

In practice, multi-architecture builds from a single workstation are uncommon
since Yoe-NG uses native builds. A developer typically builds for the
architecture of their machine. Multi-arch is more relevant in CI, where
different runners handle different architectures and share results via the
remote cache.

## Supported Host Architectures

Since Yoe-NG uses native builds (no cross-compilation), the host architecture
**is** the target architecture. All three supported architectures have viable
build environments:

| Architecture | Alpine Container        | CI Runners                      | Native Hardware         |
| ------------ | ----------------------- | ------------------------------- | ----------------------- |
| x86_64       | `alpine:latest`         | GitHub Actions, all CI          | Any x86_64 machine      |
| aarch64      | `alpine:latest` (arm64) | GitHub ARM runners, Hetzner CAX | RPi 4/5, ARM servers    |
| riscv64      | `alpine:edge` (riscv64) | Limited                         | SiFive, StarFive boards |

For riscv64, QEMU user-mode emulation on an x86_64 host is a practical fallback
until native CI runners become widely available.
