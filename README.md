# 🐧 Yoe Next Generation

Yoe-NG is an **AI-native embedded Linux distribution builder** — a simpler
alternative to Yocto, designed from the ground up to be driven by AI assistants.

Every operation has a CLI equivalent, but the primary workflow is
conversational: describe what you need, and the AI generates recipes, configures
machines, traces dependencies, diagnoses build failures, and audits security —
all with full understanding of your project's dependency graph and build state.

## 🚀 Getting Started

Prerequisites: an x86_64 Linux host with Git and Docker (or Podman) installed.

```sh
# Download the yoe binary
curl -L https://github.com/yoe/yoe-ng/releases/latest/download/yoe-Linux-x86_64 -o yoe
(project is not public yet, so download manually from releases page)
chmod +x yoe
sudo mv yoe /usr/local/bin/

# Create a new project
yoe init yoe-test
cd yoe-test

# Fetch layers (downloads recipes-core)
yoe layer sync

# Build the base image (builds all required packages, then assembles the image)
yoe build base-image

# Boot it in QEMU
yoe run base-image

# Log in a user: root, no password

# Power off when finished (inside running image)
poweroff
```

`dev-image` is another included image with a few more things in it.

## 🤖 Why AI-Native

Embedded Linux is hard not because the concepts are complex, but because there
are _many_ concepts that interact in non-obvious ways: toolchain flags,
dependency ordering, kernel configuration, package splitting, layer composition,
image assembly, device trees, bootloaders. Traditional build systems manage this
complexity through documentation that developers must read and internalize.

Yoe-NG takes a different approach: **the build system is the documentation.**
Starlark recipes are readable by both humans and AI. The dependency graph is
queryable. Build logs are structured. An AI assistant that understands all of
this can:

- **Create recipes from a URL or description** —
  `/new-recipe https://github.com/example/myapp`
- **Diagnose build failures** by reading logs and the dependency graph —
  `/diagnose openssh`
- **Trace why a package is in your image** — `/why libssl`
- **Simulate changes before building** — `/what-if remove networkmanager`
- **Audit for CVEs and license compliance** — `/cve-check`, `/license-audit`
- **Generate machine definitions from board names** —
  `/new-machine "Raspberry Pi 5"`

See [AI Skills](docs/ai-skills.md) for the full catalog of AI-driven workflows.

## 🎯 Design Priorities

- **AI-native** — structured metadata (Starlark), queryable dependency graphs,
  and AI skills as first-class interfaces. See [AI Skills](docs/ai-skills.md).
- **Three interfaces** — AI conversation, interactive TUI, and traditional CLI.
  All three do the same things; use whichever fits the moment.
- **Developer-focused** — first-class support for application development, not
  just system integration. Good tooling for kernel, applications, and BSPs
  (similar to Yocto's scope, but simpler).
- **Simple** — one Go binary, one language (Starlark), one package format (apk)
- **Easy to get started** — AI guides you through project setup, recipe
  creation, and image configuration
- **Tooling written in Go** — single static binary, no runtime dependencies, TUI
  built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), fast
  enough, trivial to distribute
- **Build dependencies isolated with bubblewrap** — no Docker daemon required,
  no host dependency pollution
- **Easy BSP support** — support for many boards, inclusive of hardware
  ecosystem
- **Global cache of pre-built assets** — minimize time building from source
- **Multiple images/targets in a single build tree** (like Yocto)
- **Rebuilding from source is first class, but not required** — fully traceable,
  no golden images
- **Modern languages** (Go, Rust, Zig, Python, JavaScript) — uses native
  language package managers, caches packages where possible
- **No cross compilation** — native builds on modern ARM/RISC-V hardware
- **Starlark for recipes and build rules** — Python-like, deterministic,
  sandboxed (see [Build Languages](docs/build-languages.md))
- **Leverage existing ecosystems** — integrate with language-native build
  systems rather than reimplementing them
- **64-bit only** — x86, ARM, RISC-V
- **Granular packaging** (like Yocto/Debian) — one recipe can produce multiple
  sub-packages (`-dev`, `-doc`, `-dbg`, custom splits)
- **Composable layers** — pull in recipes/packages using GitHub URLs; vendor
  BSP, product, and core layers compose through Starlark `load()` function calls
- **Image-based device management** — full image updates, OSTree, BDiff
- **Good SDK story** — binary SDKs, pre-built packages like Chromium

## 📚 Documentation

- [AI Skills](docs/ai-skills.md) — AI-driven workflows for recipe creation,
  build debugging, security auditing, and more
- [The `yoe` Tool](docs/yoe-tool.md) — CLI reference for building, imaging, and
  flashing
- [Recipe & Configuration Format](docs/metadata-format.md) — Starlark recipe and
  configuration spec
- [Build Environment](docs/build-environment.md) — bootstrap, host tools, and
  build isolation
- [SDK Management](docs/sdk.md) — development environments, container-based SDK,
  pre-built binary packages
- [Comparisons](docs/comparisons.md) — how Yoe-NG relates to Yocto, Buildroot,
  Alpine, Arch, and NixOS
- [Build Languages](docs/build-languages.md) — analysis of Starlark, CUE, Nix,
  and other embeddable languages for recipe definitions
- [Recipes Roadmap](docs/recipes-roadmap.md) — existing recipes and what's needed
  for a complete base system

## 💡 Inspirations

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

See [Comparisons](docs/comparisons.md) for detailed analysis of how Yoe-NG
relates to each of these systems, including when you should use them instead.

## 🔧 Motivation

The Yocto Project is a powerful embedded Linux build system, but it carries
significant complexity: BitBake, extensive metadata, cross-compilation
toolchains, and a steep learning curve. Much of this complexity exists to solve
problems that modern language ecosystems have already addressed — dependency
management, reproducible builds, and caching.

Yoe-NG asks: what if we started fresh with these assumptions?

- **AI changes the interface.** The hardest part of embedded Linux is knowing
  what to configure and how. An AI assistant that understands the build system
  can guide developers through recipe creation, debug build failures, and audit
  security — without requiring them to memorize a build system's quirks.
- **Native compilation is fast enough.** With modern hardware (including
  powerful ARM/RISC-V boards and cloud CI), cross-compilation is no longer a
  hard requirement for most workloads.
- **Language-native package managers work.** Go modules, Cargo, npm, and pip
  already handle dependency resolution and reproducibility. Wrapping them in
  another layer (recipes, bbappends) adds friction without proportional benefit.
- **Simpler tooling is better tooling.** A single Go binary with a TUI is easier
  to install, maintain, and extend than a Python-based build system with
  thousands of metadata files.
- **Structured metadata enables AI.** Starlark is deterministic, sandboxed, and
  readable by both humans and AI. Combined with a queryable dependency graph
  (`yoe desc`, `yoe refs`, `yoe graph`), the entire build state is accessible to
  AI assistants — unlike shell-based build systems where critical state is
  hidden in environment variables and implicit ordering.

## ⚙️ Design Principles

### 🚫 No Cross Compilation

Instead of maintaining cross-toolchains, Yoe-NG targets native builds:

- Build on the target architecture directly (real hardware or emulated/VM).
- Use cloud CI with native architecture runners (e.g., ARM64 GitHub Actions
  runners, Hetzner ARM boxes).
- QEMU user-mode emulation as a fallback for architectures without cheap native
  hardware.

This eliminates an entire class of build issues (sysroot management, host
contamination, cross-pkg-config, etc.).

### 📦 Native Language Package Managers

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

### 🖥️ Kernel and System Image Tooling

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

### 🏗️ Go-Based Tooling

The Yoe-NG CLI tool handles:

- **TUI** — interactive interface for common workflows (configure a build,
  select a machine, build an image, flash to SD card).
- **Build orchestration** — invoke language-native build tools in the right
  order, manage caching, assemble outputs. See
  [The `yoe` Tool](docs/yoe-tool.md) for the full CLI reference.
- **Machine/distro configuration** — define target boards and distribution
  profiles in Starlark. See
  [Recipe & Configuration Format](docs/metadata-format.md) for the full
  specification.

Why Go:

- Single static binary — no runtime dependencies, trivial to distribute.
- Fast compilation and execution.
- Excellent cross-compilation support (ironic, but useful for the tool itself).
- Strong standard library for file manipulation, process execution, and
  networking.

### 📋 Package Management: apk

Yoe-NG uses [apk](https://wiki.alpinelinux.org/wiki/Alpine_Package_Keeper)
(Alpine Package Keeper) as its package manager. It is important to distinguish
between **recipes** and **packages** — these are separate concepts:

- **Recipes** are build-time definitions (Starlark `.star` files in the project
  tree) that describe _how_ to build software. See
  [Recipe & Configuration Format](docs/metadata-format.md).
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

### 🧱 Base System

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

### 🔒 Reproducibility

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
