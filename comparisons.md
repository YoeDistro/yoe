# Comparisons

How Yoe-NG relates to existing embedded Linux build systems and distributions.
For each, we identify what Yoe-NG adopts, what it leaves behind, and where it
differs.

## vs. Yocto / OpenEmbedded

Yocto is the industry standard for custom embedded Linux. It is extremely
capable but carries significant complexity.

**What Yoe-NG adopts from Yocto:**

- **Machine abstraction** — a declarative way to define board-specific
  configuration (kernel defconfig, device tree, bootloader, partition layout).
- **Image recipes** — composable definitions of what goes into a root filesystem
  image and how it's laid out on disk.
- **Layer architecture** — the ability to overlay vendor BSP customizations on
  top of a common base without forking.
- **OTA integration** — first-class support for update frameworks (RAUC,
  SWUpdate).

**What Yoe-NG leaves behind:**

- BitBake and the task-level dependency graph.
- The recipe/bbappend/bbclass metadata system.
- sstate-cache complexity.
- Cross-compilation toolchains.
- Python as the tooling language.

**Key differences:**

|                     | Yocto                                        | Yoe-NG                                        |
| ------------------- | -------------------------------------------- | --------------------------------------------- |
| Build system        | BitBake (Python)                             | `yoe` (Go)                                    |
| Package format      | rpm / deb / ipk                              | apk                                           |
| Config format       | BitBake recipes (.bb/.bbappend)              | TOML                                          |
| Cross-compilation   | Required, central design assumption          | None — native builds only                     |
| Dependency model    | Task-level DAG (do_fetch → do_compile → ...) | Recipe-level DAG (simpler, atomic per-recipe) |
| Language ecosystems | Wrapped in recipes                           | Native toolchains (go modules, cargo, etc.)   |
| Learning curve      | Steep — weeks to become productive           | Shallow — TOML + shell commands               |
| Build caching       | sstate (per-task, hash-based)                | Per-recipe content-addressed `.apk` hashes    |
| On-device updates   | Possible but complex (smart image)           | Built-in via apk repositories                 |

**When to use Yocto instead:** when you need extremely fine-grained control over
every component, must support exotic architectures with no native build
infrastructure, or are in an organization that already has deep Yocto expertise
and tooling invested.

## vs. Buildroot

Buildroot is the simplest of the established embedded Linux build systems. It
shares Yoe-NG's preference for simplicity.

**What Yoe-NG adopts from Buildroot:**

- The principle that simpler is better.
- Minimal base system approach.

**What Yoe-NG leaves behind:**

- Kconfig as the configuration interface.
- Make as the build engine.
- The assumption that cross-compilation is required.
- Full-rebuild-on-config-change behavior.

**Key differences:**

|                    | Buildroot                                     | Yoe-NG                                              |
| ------------------ | --------------------------------------------- | --------------------------------------------------- |
| Configuration      | Kconfig (menuconfig)                          | TOML files                                          |
| Build engine       | Make                                          | `yoe` (Go)                                          |
| Cross-compilation  | Required                                      | None — native builds only                           |
| On-device packages | None — monolithic image only                  | apk — incremental updates                           |
| Incremental builds | Limited — config change triggers full rebuild | Content-addressed cache, only rebuild what changed  |
| Modern languages   | Wraps Go/Rust/etc. in Make, often poorly      | Delegates to native toolchains                      |
| Build caching      | ccache at best, no output caching             | Content-addressed `.apk` cache, shareable across CI |
| CI/team sharing    | Everyone rebuilds from scratch                | Push/pull from shared package repo                  |
| Composable images  | No — single image output                      | Yes — assemble different images from same packages  |

**The biggest structural difference** is the recipe/package split. Buildroot
has no concept of installable packages — it builds everything into a monolithic
rootfs. This means:

- You can't update a single component on a deployed device without reflashing.
- You can't share build outputs between developers or CI runs.
- You can't compose different images from the same set of built packages.

**When to use Buildroot instead:** when you want the absolute simplest build
system for a truly minimal, single-purpose, static embedded system (firmware for
a sensor, a network appliance with no field updates). If the device never needs
a partial update and the image is small enough to rebuild in minutes, Buildroot's
simplicity is hard to beat.

## vs. Alpine Linux

Alpine is the closest existing distribution to what Yoe-NG's target runtime
looks like.

**What Yoe-NG adopts from Alpine:**

- **apk as the package manager** — adopted directly. Fast, simple, proven.
- **busybox as coreutils** — minimal userspace in a single binary.
- **Minimal base image size** — target single-digit MB base images before
  application payload.
- **Security-conscious defaults** — no unnecessary services, no open ports, no
  setuid binaries unless explicitly required.
- **Fast package operations** — install/remove measured in milliseconds.

**What Yoe-NG leaves behind:**

- **musl** — using glibc instead for maximum compatibility with language runtimes
  and pre-built binaries.
- **No systemd** — Alpine uses OpenRC; Yoe-NG uses systemd.
- **Limited BSP/hardware story** — Alpine doesn't target custom embedded boards.

**Key differences:**

|                   | Alpine                            | Yoe-NG                                       |
| ----------------- | --------------------------------- | -------------------------------------------- |
| C library         | musl                              | glibc                                        |
| Init system       | OpenRC                            | systemd                                      |
| Target            | Containers, small servers         | Custom embedded hardware                     |
| BSP support       | Generic x86/ARM images            | Per-board machine definitions                |
| Image assembly    | `alpine-make-rootfs`              | `yoe image` with machine + partition support |
| Build system      | `abuild` + APKBUILD shell scripts | `yoe build` + TOML recipes                   |
| Kernel management | Generic kernels                   | Per-machine kernel config, device trees      |
| OTA updates       | Standard apk upgrade              | apk + full image update + rollback           |

**When to use Alpine instead:** when you're targeting containers or generic
server hardware and don't need custom BSP, kernel configuration, or image
assembly tooling. Alpine is an excellent base for Docker containers and small
VMs.

## vs. Arch Linux

Arch is a philosophy as much as a distribution. Its commitment to simplicity and
transparency directly influences Yoe-NG's design.

**What Yoe-NG adopts from Arch:**

- **Rolling release model** — no big-bang version upgrades; packages update
  continuously against a single branch.
- **Minimal base, user-assembled** — ship the smallest useful system and let
  the integrator compose what they need.
- **PKGBUILD-style simplicity** — build definitions should be concise, readable
  shell-like scripts, not complex metadata. Yoe-NG's TOML recipes aim for
  similar auditability.
- **Documentation culture** — invest in clear, practical docs rather than tribal
  knowledge.

**What Yoe-NG leaves behind:**

- x86-centric assumptions.
- pacman (using apk instead).
- The expectation of interactive manual system administration.
- Lack of reproducibility guarantees.

**Key differences:**

|                   | Arch                      | Yoe-NG                      |
| ----------------- | ------------------------- | --------------------------- |
| Target            | Desktop/server, x86-first | Embedded, multi-arch        |
| Package manager   | pacman                    | apk                         |
| Package format    | tar.zst + .PKGINFO        | apk (tar.gz + .PKGINFO)     |
| Build definitions | PKGBUILD (bash)           | TOML recipes                |
| Reproducibility   | Not a goal                | Content-addressed builds    |
| Image assembly    | Manual (pacstrap)         | Automated (`yoe image`)     |
| Administration    | Interactive (hands-on)    | Declarative (config-driven) |

**When to use Arch instead:** when you're building a desktop or server system
for personal use and value having full manual control. Arch's philosophy works
well for power users on general-purpose hardware.

## vs. NixOS / Nix

Nix is the most intellectually ambitious of the systems Yoe-NG draws from.
Its ideas about reproducibility and declarative configuration are adopted
wholesale; its implementation complexity is not.

**What Yoe-NG adopts from Nix:**

- **Content-addressed build cache** — build outputs keyed by their inputs so
  identical builds produce cache hits regardless of when or where they run.
- **Declarative system configuration** — the entire system image is defined by
  configuration files; rebuilding from that config produces the same result.
- **Hermetic builds** — builds do not depend on ambient host state; inputs are
  explicit and pinned.
- **Atomic system updates and rollback** — deploy new system images atomically
  with the ability to boot into the previous version.

**What Yoe-NG leaves behind:**

- The Nix expression language.
- The `/nix/store` path model and its massive closure sizes.
- The steep learning curve.
- The assumption of abundant disk space and bandwidth.

**Key differences:**

|                 | NixOS                                | Yoe-NG                                     |
| --------------- | ------------------------------------ | ------------------------------------------ |
| Config language | Nix (custom functional language)     | TOML                                       |
| Store model     | Content-addressed `/nix/store` paths | Standard FHS with apk                      |
| Closure size    | Often 1GB+ for simple systems        | Target single-digit MB base                |
| Target          | Desktop, server, CI                  | Embedded hardware                          |
| BSP support     | Minimal                              | Per-board machine definitions              |
| Package manager | Nix                                  | apk                                        |
| Reproducibility | Bit-for-bit (aspirational)           | Content-addressed, functionally equivalent |
| Rollback        | Via Nix generations                  | Via A/B partitions or apk                  |
| Learning curve  | Steep (must learn Nix language)      | Shallow (TOML + shell)                     |

**When to use Nix instead:** when you need the strongest possible
reproducibility guarantees, are building for desktop/server/CI, and are willing
to invest in learning the Nix ecosystem. NixOS is unmatched for declarative
system management on general-purpose hardware.

## vs. Google GN

GN is not a Linux distribution — it's a meta-build system used by Chromium and
Fuchsia. But several of its architectural ideas directly influenced Yoe-NG's
tooling design.

**What Yoe-NG adopts from GN:**

- **Two-phase resolve-then-build** — GN fully resolves and validates the
  dependency graph before generating any build files. `yoe build` does the
  same: resolve the entire recipe DAG, check for errors, then build. No partial
  builds from graph errors discovered mid-way.
- **Config propagation** — GN's `public_configs` automatically apply compiler
  flags to anything that depends on a target. Yoe-NG propagates machine-level
  settings (arch flags, optimization, kernel headers) through the recipe graph.
- **Build introspection** — GN provides `gn desc` (what does this target do?)
  and `gn refs` (what depends on this?). Yoe-NG provides `yoe desc`,
  `yoe refs`, and `yoe graph` for the same purpose.
- **Label-based references** — GN uses `//path/to:target` for unambiguous
  target identification. Yoe-NG uses a similar scheme for composable recipe
  references across repositories.

**What Yoe-NG leaves behind:**

- Ninja file generation — Yoe-NG's recipe builds are coarse-grained enough
  that `yoe` orchestrates directly.
- GN's custom scripting language — TOML is sufficient for Yoe-NG's metadata.
- C/C++ build model specifics — GN is deeply tied to source-file-level
  dependency tracking, which isn't relevant for recipe-level builds.

**Key differences:**

| | GN | Yoe-NG |
|---|---|---|
| Purpose | C/C++ meta-build system | Embedded Linux distribution builder |
| Output | Ninja build files | `.apk` packages and disk images |
| Config language | GN (custom) | TOML |
| Dependency granularity | Source file / target | Recipe (package) |
| Build execution | Ninja | `yoe` directly |
| Introspection | `gn desc`, `gn refs` | `yoe desc`, `yoe refs`, `yoe graph` |

GN is not an alternative to Yoe-NG — they solve different problems. But GN's
approach to graph resolution, config propagation, and introspection are
well-proven patterns that Yoe-NG applies to the embedded Linux domain.

## Summary Matrix

| Feature                 | Yocto    | Buildroot | Alpine   | Arch     | NixOS   | **Yoe-NG** |
| ----------------------- | -------- | --------- | -------- | -------- | ------- | ---------- |
| Embedded focus          | Yes      | Yes       | Partial  | No       | No      | **Yes**    |
| Simple config           | No       | Moderate  | Moderate | Yes      | No      | **Yes**    |
| Native builds           | No       | No        | Yes      | Yes      | Yes     | **Yes**    |
| On-device packages      | Optional | No        | Yes      | Yes      | Yes     | **Yes**    |
| Content-addressed cache | Partial  | No        | No       | No       | Yes     | **Yes**    |
| Declarative images      | Yes      | Partial   | No       | No       | Yes     | **Yes**    |
| Custom BSP support      | Yes      | Yes       | No       | No       | Minimal | **Yes**    |
| Incremental updates     | Complex  | No        | Yes      | Yes      | Yes     | **Yes**    |
| Hermetic builds         | Partial  | No        | No       | No       | Yes     | **Yes**    |
| Fast package ops        | N/A      | N/A       | Yes      | Moderate | Slow    | **Yes**    |
