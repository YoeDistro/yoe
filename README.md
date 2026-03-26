# Yoe Next Generation

Yoe-NG is a thinking exercise to explore how a Linux distribution might look
with the following priorities:

- Simplicity
- Focused on developer (including app developer) usability
  - First class support for application development.
- Easy to get started
- Build dependencies distributed through apk packages, isolated with bubblewrap
  — no Docker daemon required, no host dependency pollution.
- Easy BSP support
  - Support for a lot of boards
  - Inclusive
- Global cache of pre-build assets
  - Minimize time building from source
- Support for multiple images/targets in a single build tree (like Yocto)
- Rebuilding from source target packages is first class, but not required
  - Fully traceable
  - No golden images
- Focused on modern languages (Go, Rust, Zig, Python, JavaScript)
  - Uses native language package managers
  - Caches packages where possible
- No cross compilation
- Good tooling for kernel and applications (similar to Yocto)
- Tooling is written in Go
  - TUI for common operations
  - Fast enough
  - Simple
  - Easy to cross-compile
  - Starlark for recipes and build rules (see
    [Build Languages](build-languages.md))
- Leverage knowledge and build systems that already exist and integrate with
  them.
- 64-bit only (no 32-bit)
- x86, ARM, RISC-V only
- Granular packaging (like Yocto/Debian) — one recipe can produce multiple
  sub-packages (`-dev`, `-doc`, `-dbg`, custom splits), keeping production
  images small while development images stay fully featured
- Composable
  - Pull in recipes/packages using GitHub URLs
  - Layer composition via Starlark `load()` — vendor BSP, product, and core
    layers compose through function calls, not config file merging
  - Recipes can build packages, tools, images, everything
- Primarily Image based device management (vs package based)
  - Full image updates, OSTree, BDiff (Android uses)
- Good SDK story
  - Able to distribute Binary SDKs to quickly get going with builds without full
    rebuilds.
  - Able distribute large pre-built packages like Chromium

## Documentation

- [The `yoe` Tool](yoe-tool.md) — CLI reference for building, imaging, and
  flashing
- [Recipe & Configuration Format](metadata-format.md) — Starlark recipe and
  configuration spec
- [Build Environment](build-environment.md) — bootstrap, host tools, and build
  isolation
- [SDK Management](sdk.md) — development environments, container-based SDK,
  pre-built binary packages
- [Comparisons](comparisons.md) — how Yoe-NG relates to Yocto, Buildroot,
  Alpine, Arch, and NixOS
- [Build Languages](build-languages.md) — analysis of Starlark, CUE, Nix, and
  other embeddable languages for recipe definitions

## Inspirations

Yoe-NG draws selectively from five existing systems, taking the best ideas from
each while avoiding their respective pain points:

- **Yocto** — machine abstraction, image composition, layer architecture, OTA
  integration. Leave behind BitBake, sstate, cross-compilation complexity.
- **Buildroot** — the principle that simpler is better. Leave behind monolithic
  images and full-rebuild-on-config-change.
- **Arch** — rolling release, minimal base, PKGBUILD-style simplicity,
  documentation culture. Leave behind x86-centrism and manual administration.
- **Alpine** — apk package manager, busybox, minimal footprint, security
  defaults. Leave behind musl and lack of BSP support.
- **Nix** — content-addressed caching, declarative configuration, hermetic
  builds, atomic rollback. Leave behind the Nix language and store-path
  complexity.
- **Google GN** — two-phase resolve-then-build model, config propagation through
  the dependency graph, build introspection commands, label-based target
  references for composability. Leave behind the C++-specific build model and
  Ninja generation.

See [Comparisons](comparisons.md) for detailed analysis of how Yoe-NG relates to
each of these systems, including when you should use them instead.

## Motivation

The Yocto Project is a powerful embedded Linux build system, but it carries
significant complexity: BitBake, extensive metadata, cross-compilation
toolchains, and a steep learning curve. Much of this complexity exists to solve
problems that modern language ecosystems have already addressed — dependency
management, reproducible builds, and caching.

Yoe-NG asks: what if we started fresh with these assumptions?

- **Native compilation is fast enough.** With modern hardware (including
  powerful ARM/RISC-V boards and cloud CI), cross-compilation is no longer a
  hard requirement for most workloads.
- **Language-native package managers work.** Go modules, Cargo, npm, and pip
  already handle dependency resolution and reproducibility. Wrapping them in
  another layer (recipes, bbappends) adds friction without proportional benefit.
- **Simpler tooling is better tooling.** A single Go binary with a TUI is easier
  to install, maintain, and extend than a Python-based build system with
  thousands of metadata files.

## Design Principles

### No Cross Compilation

Instead of maintaining cross-toolchains, Yoe-NG targets native builds:

- Build on the target architecture directly (real hardware or emulated/VM).
- Use cloud CI with native architecture runners (e.g., ARM64 GitHub Actions
  runners, Hetzner ARM boxes).
- QEMU user-mode emulation as a fallback for architectures without cheap native
  hardware.

This eliminates an entire class of build issues (sysroot management, host
contamination, cross-pkg-config, etc.).

### Native Language Package Managers

Each language ecosystem manages its own dependencies:

| Language   | Package Manager | Lock File           |
| ---------- | --------------- | ------------------- |
| Go         | Go modules      | `go.sum`            |
| Rust       | Cargo           | `Cargo.lock`        |
| Python     | pip / uv        | `requirements.lock` |
| JavaScript | npm / pnpm      | `package-lock.json` |
| Zig        | Zig build       | `build.zig.zon`     |

Yoe-NG provides caching infrastructure (a shared module proxy for Go, a registry
mirror for Cargo/npm, etc.) so builds are fast and repeatable without
re-downloading the internet.

### Kernel and System Image Tooling

While application builds use native language tooling, the system-level pieces
still need orchestration:

- **Kernel builds** — configure, build, and package kernels for target boards.
- **Root filesystem assembly** — combine built artifacts into a bootable image
  (ext4, squashfs, etc.).
- **Device tree / bootloader management** — board-specific configuration.
- **OTA / update support** — integration with update frameworks (RAUC, SWUpdate,
  etc.).

This is where Yoe-NG tooling (written in Go) provides value — similar to what
`bitbake` and `wic` do in Yocto, but simpler and more opinionated.

### Go-Based Tooling

The Yoe-NG CLI tool handles:

- **TUI** — interactive interface for common workflows (configure a build,
  select a machine, build an image, flash to SD card).
- **Build orchestration** — invoke language-native build tools in the right
  order, manage caching, assemble outputs. See [The `yoe` Tool](yoe-tool.md) for
  the full CLI reference.
- **Machine/distro configuration** — define target boards and distribution
  profiles in Starlark. See [Recipe & Configuration Format](metadata-format.md)
  for the full specification.

Why Go:

- Single static binary — no runtime dependencies, trivial to distribute.
- Fast compilation and execution.
- Excellent cross-compilation support (ironic, but useful for the tool itself).
- Strong standard library for file manipulation, process execution, and
  networking.

### Package Management: apk

Yoe-NG uses [apk](https://wiki.alpinelinux.org/wiki/Alpine_Package_Keeper)
(Alpine Package Keeper) as its package manager. It is important to distinguish
between **recipes** and **packages** — these are separate concepts:

- **Recipes** are build-time definitions (Starlark `.star` files in the project
  tree) that describe _how_ to build software. See
  [Recipe & Configuration Format](metadata-format.md).
- **Packages** are installable artifacts (`.apk` files) that recipes produce.
  They are what gets installed into root filesystem images and onto devices.

This separation means recipes are a development/CI concern, while packages are a
deployment/device concern. You can build packages once and install them on many
devices without needing the recipe tree.

Why apk over pacman or opkg:

- **Speed** — apk operations are near-instantaneous. Install, remove, and
  upgrade are measured in milliseconds, not seconds.
- **Simple format** — an `.apk` package is a signed tar.gz with a `.PKGINFO`
  metadata file. No complex archive-in-archive wrapping.
- **Small footprint** — apk-tools is tiny, appropriate for embedded targets.
- **Active development** — apk 3.x adds content-addressed storage and atomic
  transactions, aligning with Yoe-NG's Nix-inspired reproducibility goals.
- **Works with glibc** — apk is not tied to musl; it works with any libc. Yoe-NG
  runs its own package repositories, not Alpine's.
- **On-device package management** — devices can pull updates from a Yoe-NG
  package repository, enabling incremental OTA updates (install only changed
  packages) alongside full image updates.

The Yoe-NG build tooling invokes recipes to produce `.apk` packages, which are
published to a repository. Image assembly then uses `apk` to install packages
into a root filesystem, just as Alpine does.

### Base System

The base userspace is **glibc + busybox + systemd**:

- **glibc** — the standard C library. Maximizes compatibility with pre-built
  binaries, language runtimes (Go, Rust, Python, Node.js), and third-party
  libraries. musl is lighter but introduces subtle compatibility issues that
  aren't worth fighting in a system that already includes systemd.
- **busybox** — provides the core userspace utilities (sh, coreutils, etc.) in a
  single small binary. Keeps the base image minimal while still having a
  functional shell environment for debugging and scripting.
- **systemd** — the init system and service manager. Despite its size, systemd
  is the pragmatic choice:
  - Well-understood by developers and ops teams.
  - Rich ecosystem of unit files for common services.
  - Built-in support for journal logging, network management, device management
    (udev), and container integration.
  - Required or assumed by many modern Linux components.

This combination gives a small but fully functional base system that can run
real-world services without surprises.

### Reproducibility

Yoe-NG targets **functional equivalence**, not bit-for-bit reproducibility. Same
inputs produce functionally identical outputs — same behavior, same files, same
permissions — but the bytes may differ due to embedded timestamps, archive
member ordering, or compiler non-determinism.

This is a deliberate trade-off:

- **Bit-for-bit reproducibility** (what Nix aspires to) requires patching
  upstream build systems to eliminate timestamps (`__DATE__`, `.pyc` mtime),
  enforce file ordering in archives, and strip or fix build IDs. This is
  enormous effort — Nix still hasn't fully achieved it after 20 years — and the
  primary benefit (verifying a binary matches its source by rebuilding) is
  relevant mainly for high-assurance supply-chain contexts.
- **Functional equivalence** gets the practical benefits — reliable caching,
  hermetic builds, provenance tracking — without the patching burden. Bubblewrap
  isolation prevents host contamination. Content-addressed input hashing (recipe
  - source + dependency hashes) ensures cache hits are reliable. Starlark
    evaluation is deterministic by design. The remaining non-determinism
    (timestamps, ordering within packages) doesn't affect functionality or
    caching.

The caching model does not depend on output determinism. Cache keys are computed
from _inputs_ (recipe content, source hash, dependency `.apk` hashes, build
flags), not _outputs_. If inputs haven't changed, the cached output is used
regardless of whether a fresh build would produce identical bytes.
