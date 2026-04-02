# 🐧 Yoe Next Generation

**For teams building edge products in Go, Rust, Zig, Python, etc. who need Linux
on ARM/RISC-V without the complexity of Yocto.**

Yoe-NG is an embedded Linux distribution builder designed for modern edge
development. Your application is written in Go or Rust. Your target is an ARM
board. You need a minimal Linux image with your app, the right kernel, and
nothing else. You shouldn't need to learn BitBake, manage cross-toolchains, or
read a thousand-page manual to get there.

One Go binary. Readable Starlark config files. AI that understands your
dependency graph. Build ARM images on your x86 laptop, on native hardware, or in
cloud CI — same tool, same config, same results.

Note: much of what is in the docs has not been implemented yet, and is mostly
vision.

## 🚀 Getting Started

Prerequisites: Linux or macOS with Git and Docker (or Podman) installed. Windows
users: install WSL2 and use the Linux binary. (Linux x86_64/Docker is the most
tested configuration)

```sh
# Download the yoe binary
curl -L https://github.com/YoeDistro/yoe/releases/latest/download/yoe-$(uname -s)-$(uname -m) -o yoe

chmod +x yoe
mkdir -p ~/bin
mv yoe ~/bin/
# Make sure ~/bin is in your PATH (add to ~/.bashrc or ~/.zshrc if needed)
export PATH="$HOME/bin:$PATH"

# Create a new project
yoe init yoe-test
cd yoe-test

# start the TUI (see screenshot below)
yoe

# navigate to the base-image and press 'b' to build.

# when build is complete, press 'r' to run.

# Log in a user: root, no password

# Power off when finished (inside running image)
poweroff
```

There are also CLI variants of the above commands (`build`, `run`, etc.).

<img width="1743" height="1597" alt="image" src="https://github.com/user-attachments/assets/99e297f3-b424-422a-8b24-45fb82de81fb" />

`dev-image` is another included image with a few more things in it.

**What just happened:**

1. `yoe init` created a project with a `PROJECT.star` config and a default
   x86_64 QEMU machine.
2. On first build, `yoe` automatically built a Docker container with the
   toolchain (gcc, make, etc.) and fetched the `units-core` module from GitHub.
3. It built ~10 packages from source (busybox, linux kernel, openssl, etc.)
   inside the container, each isolated in its own bubblewrap sandbox.
4. It assembled a bootable disk image from those packages.
5. `yoe run` launched the image in QEMU with KVM acceleration.

Everything is in the project directory — no global state, no hidden caches
outside the tree.

### Cross-Architecture Builds

Build ARM64 images on an x86_64 host using QEMU user-mode emulation:

```sh
# One-time setup: register QEMU user-mode emulation
yoe container binfmt

# Build for ARM64
yoe build base-image --machine qemu-arm64

# Run it
yoe run base-image --machine qemu-arm64
```

(or run the above in TUI be selecting machine in setup first)

No cross-compilation toolchain needed — the build runs inside a genuine ARM64
Docker container, transparently emulated by the host kernel.

## 🔧 Motivation

Existing embedded Linux build systems (Yocto, Buildroot) were designed in a
world where ARM hardware was slow, applications were written in C, and
developers configured everything by hand. Three things have changed:

1. **ARM and RISC-V hardware is fast.** Modern ARM boards and cloud instances
   (AWS Graviton, Hetzner CAX) build at speeds that make cross-compilation
   unnecessary for most workloads. For development, QEMU user-mode emulation
   lets you build ARM images on x86 without a cross-toolchain — slower, but
   correct and simple.

2. **Applications are moving to modern languages.** Go, Rust, Zig, and Python
   have their own dependency management, reproducible builds, and caching. The
   elaborate cross-compilation and sysroot machinery in traditional build
   systems was designed for C/C++ — wrapping Go modules or Cargo in BitBake
   recipes adds friction without proportional benefit.

3. **AI changes the interface.** The hardest part of embedded Linux is knowing
   what to configure and how. An AI assistant that understands the build system
   can guide developers through unit creation, debug build failures, and audit
   security — without requiring them to memorize a build system's quirks. But
   this only works if the build metadata is structured and queryable, not buried
   in shell scripts and environment variables.

Yoe-NG is built for this new world: native builds, language-native package
managers, structured Starlark metadata, and AI as a first-class interface.

## 🤖 Why AI-Native

Embedded Linux is hard not because the concepts are complex, but because there
are _many_ concepts that interact in non-obvious ways: toolchain flags,
dependency ordering, kernel configuration, package splitting, module
composition, image assembly, device trees, bootloaders. Traditional build
systems manage this complexity through complexity.

Yoe-NG takes a different approach: **Simplify things as much as possible.**
Starlark units are readable by both humans and AI. The dependency graph is
queryable. Build logs are structured. An AI assistant that understands all of
this can:

- **Create units from a URL or description** —
  `/new-unit https://github.com/example/myapp`
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
- **Easy to get started** — AI guides you through project setup, unit creation,
  and image configuration
- **Tooling written in Go** — single static binary, no runtime dependencies, TUI
  built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), fast
  enough, trivial to distribute
- **Build dependencies isolated with bubblewrap** — no host dependency pollution
- **Easy BSP support** — support for many boards, inclusive of hardware
  ecosystem
- **Global cache of pre-built assets** — minimize time building from source
- **Multiple images/targets in a single build tree** (like Yocto)
- **Rebuilding from source is first class, but not required** — fully traceable,
  no golden images
- **Modern languages** (Go, Rust, Zig, Python, JavaScript) — uses native
  language package managers, caches packages where possible
- **No cross compilation** — native builds via QEMU user-mode emulation or real
  ARM/RISC-V hardware. Build environment is per-unit, not global — each unit
  runs in its own isolated container.
- **Starlark for units and build rules** — Python-like, deterministic, sandboxed
  (see [Build Languages](docs/build-languages.md))
- **Leverage existing ecosystems** — integrate with language-native build
  systems rather than reimplementing them
- **64-bit only** — x86, ARM, RISC-V
- **Granular packaging** (like Yocto/Debian) — one unit can produce multiple
  sub-packages (`-dev`, `-doc`, `-dbg`, custom splits)
- **Composable modules** — pull in units/packages using GitHub URLs; vendor BSP,
  product, and core modules compose through Starlark `load()` function calls
- **Image-based device management** — full image updates, OSTree, BDiff
- **Good SDK story** — binary SDKs, pre-built packages like Chromium
- **Parallel** — no global lock or global resource, support running concurrent
  versions of `yoe` concurrently. This is essential for rapid development using
  AI.

## 📚 Documentation

- [AI Skills](docs/ai-skills.md) — AI-driven workflows for unit creation, build
  debugging, security auditing, and more
- [The `yoe` Tool](docs/yoe-tool.md) — CLI reference for building, imaging, and
  flashing
- [Unit & Configuration Format](docs/metadata-format.md) — Starlark unit and
  configuration spec
- [Build Environment](docs/build-environment.md) — bootstrap, host tools, and
  build isolation
- [SDK Management](docs/sdk.md) — development environments, container-based SDK,
  pre-built binary packages
- [Comparisons](docs/comparisons.md) — how Yoe-NG relates to Yocto, Buildroot,
  Alpine, Arch, and NixOS
- [Build Languages](docs/build-languages.md) — analysis of Starlark, CUE, Nix,
  and other embeddable languages for unit definitions
- [Units Roadmap](docs/units-roadmap.md) — existing units and what's needed for
  a complete base system

## 💡 Inspirations

Yoe-NG draws selectively from five existing systems, taking the best ideas from
each while avoiding their respective pain points:

- **Yocto** — machine abstraction, image composition, module architecture, OTA
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

## ⚙️ Design Principles

### 🚫 No Cross Compilation

Instead of maintaining cross-toolchains, Yoe-NG targets native builds:

- **QEMU user-mode emulation** — build ARM64 or RISC-V images on any x86_64
  workstation. The build runs inside a genuine foreign-arch Docker container,
  transparently emulated via binfmt_misc. One command to set up
  (`yoe container binfmt`), then `--machine qemu-arm64` just works. ~5-20x
  slower than native, but fine for iterating on a few packages.
- **Native hardware** — build on the target architecture directly (ARM64 dev
  boards, RISC-V boards).
- **Cloud CI** — use native architecture runners (e.g., ARM64 GitHub Actions
  runners, AWS Graviton, Hetzner ARM boxes) for full-speed CI builds.

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

- **TUI** — run `yoe` with no arguments for an interactive unit list with inline
  build status, background builds, search, and quick actions (edit, diagnose,
  clean).
- **Build orchestration** — invoke language-native build tools in the right
  order, manage caching, assemble outputs. See
  [The `yoe` Tool](docs/yoe-tool.md) for the full CLI reference.
- **Machine/distro configuration** — define target boards and distribution
  profiles in Starlark. See
  [Unit & Configuration Format](docs/metadata-format.md) for the full
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
between **units** and **packages** — these are separate concepts:

- **Units** are build-time definitions (Starlark `.star` files in the project
  tree) that describe _how_ to build software. See
  [Unit & Configuration Format](docs/metadata-format.md).
- **Packages** are installable artifacts (`.apk` files) that units produce. They
  are what gets installed into root filesystem images and onto devices.

This separation means units are a development/CI concern, while packages are a
deployment/device concern. You can build packages once and install them on many
devices without needing the unit tree.

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

The Yoe-NG build tooling invokes units to produce `.apk` packages, which are
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
  isolation prevents host contamination. Content-addressed input hashing (unit
  - source + dependency hashes) ensures cache hits are reliable. Starlark
    evaluation is deterministic by design. The remaining non-determinism
    (timestamps, ordering within packages) doesn't affect functionality or
    caching.

The caching model does not depend on output determinism. Cache keys are computed
from _inputs_ (unit content, source hash, dependency `.apk` hashes, build
flags), not _outputs_. If inputs haven't changed, the cached output is used
regardless of whether a fresh build would produce identical bytes.
