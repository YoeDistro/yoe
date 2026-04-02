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
- **Image units** — composable definitions of what goes into a root filesystem
  image and how it's laid out on disk.
- **Module architecture** — the ability to overlay vendor BSP customizations on
  top of a common base without forking.
- **OTA integration** — first-class support for update frameworks (RAUC,
  SWUpdate).

**What Yoe-NG leaves behind:**

- BitBake and the task-level dependency graph.
- The unit/bbappend/bbclass metadata system.
- sstate-cache complexity — Yocto's sstate is per-task and requires careful
  configuration of mirrors, hash equivalence servers, and signing. Yoe-NG's
  cache is per-unit, stored in S3-compatible object storage, and needs only a
  bucket URL.
- Cross-compilation toolchains.
- Python as the tooling language.

**No conditional override syntax.** Yocto's
[override system](https://docs.yoctoproject.org/bitbake/bitbake-user-manual/bitbake-user-manual-metadata.html#conditional-syntax-overrides)
(`DEPENDS:append:raspberrypi4`, `SRC_URI:remove:aarch64`, etc.) exists because
BitBake's metadata model is variable-based — you set global variables and then
layer conditional string operations on top. The result is powerful but
notoriously hard to debug (you need `bitbake -e` to see what a variable actually
resolved to).

Yoe-NG's model is function-based, which covers the same use cases more
explicitly:

| Yocto override                     | Yoe-NG equivalent                                         |
| ---------------------------------- | --------------------------------------------------------- |
| `DEPENDS:append:raspberrypi4`      | `if MACHINE == "raspberrypi4": extra_deps = [...]`        |
| `SRC_URI:append:aarch64`           | `if ARCH == "aarch64": ...` in the unit                   |
| `PACKAGECONFIG:remove:musl`        | Module scoping — musl project doesn't include that module |
| `FILESEXTRAPATHS:prepend` + append | `load()` the upstream function, call with different args  |

Starlark has `if` with predeclared variables (`MACHINE`, `ARCH`), and the
function composition pattern handles the "extend from downstream" case. When
machine-specific behavior is needed, it's right there in the `.star` file — no
hidden layering of string operations.

**Key differences:**

|                     | Yocto                                        | Yoe-NG                                        |
| ------------------- | -------------------------------------------- | --------------------------------------------- |
| Build system        | BitBake (Python)                             | `yoe` (Go)                                    |
| Package format      | rpm / deb / ipk                              | apk                                           |
| Config format       | BitBake units (.bb/.bbappend)                | Starlark (Python-like)                        |
| Cross-compilation   | Required, central design assumption          | None — native builds only                     |
| Dependency model    | Task-level DAG (do_fetch → do_compile → ...) | Unit-level DAG (simpler, atomic per-unit)     |
| Language ecosystems | Wrapped in units                             | Native toolchains (go modules, cargo, etc.)   |
| Learning curve      | Steep — weeks to become productive           | Shallow — Starlark (Python-like)              |
| Build caching       | sstate (per-task, hash-based, complex setup) | Per-unit `.apk` hashes in S3-compatible cache |
| Multi-image support | Yes — multiple images from one project       | Yes — image inheritance + machine matrix      |
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
| Configuration      | Kconfig (menuconfig)                          | Starlark files                                      |
| Build engine       | Make                                          | `yoe` (Go)                                          |
| Cross-compilation  | Required                                      | None — native builds only                           |
| On-device packages | None — monolithic image only                  | apk — incremental updates                           |
| Incremental builds | Limited — config change triggers full rebuild | Content-addressed cache, only rebuild what changed  |
| Modern languages   | Wraps Go/Rust/etc. in Make, often poorly      | Delegates to native toolchains                      |
| Build caching      | ccache at best, no output caching             | Content-addressed `.apk` cache, shareable across CI |
| CI/team sharing    | Everyone rebuilds from scratch                | Push/pull from shared package repo                  |
| Composable images  | No — single image output                      | Yes — assemble different images from same packages  |

**The biggest structural difference** is the unit/package split. Buildroot has
no concept of installable packages — it builds everything into a monolithic
rootfs. This means:

- You can't update a single component on a deployed device without reflashing.
- You can't share build outputs between developers or CI runs.
- You can't compose different images from the same set of built packages.

**Caching gap:** Buildroot has no output caching at all — every developer and
every CI run rebuilds from source. `ccache` can help with C/C++ compilation but
doesn't help with configure steps, language-native builds, or package assembly.
Yoe-NG's S3-backed cache means a typical developer build pulls pre-built
packages for everything except the component they're actively changing.

**Multi-image gap:** Buildroot produces a single image per configuration. To
build a "dev" variant and a "production" variant, you need separate build
directories with separate configs. With Yoe-NG, both images share the same
package repository — only the package lists differ.

**When to use Buildroot instead:** when you want the absolute simplest build
system for a truly minimal, single-purpose, static embedded system (firmware for
a sensor, a network appliance with no field updates). If the device never needs
a partial update and the image is small enough to rebuild in minutes,
Buildroot's simplicity is hard to beat.

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

- **musl** — using glibc instead for maximum compatibility with language
  runtimes and pre-built binaries.
- **No systemd** — Alpine uses OpenRC; Yoe-NG uses systemd.
- **Limited BSP/hardware story** — Alpine doesn't target custom embedded boards.

**Key differences:**

|                   | Alpine                            | Yoe-NG                                               |
| ----------------- | --------------------------------- | ---------------------------------------------------- |
| C library         | musl                              | glibc                                                |
| Init system       | OpenRC                            | systemd                                              |
| Target            | Containers, small servers         | Custom embedded hardware                             |
| BSP support       | Generic x86/ARM images            | Per-board machine definitions                        |
| Image assembly    | `alpine-make-rootfs`              | `yoe build <image>` with machine + partition support |
| Build system      | `abuild` + APKBUILD shell scripts | `yoe build` + Starlark units                         |
| Kernel management | Generic kernels                   | Per-machine kernel config, device trees              |
| OTA updates       | Standard apk upgrade              | apk + full image update + rollback                   |

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
- **Minimal base, user-assembled** — ship the smallest useful system and let the
  integrator compose what they need.
- **PKGBUILD-style simplicity** — build definitions should be concise, readable
  shell-like scripts, not complex metadata. Yoe-NG's Starlark units aim for
  similar auditability — simple units read like declarative config.
- **Documentation culture** — invest in clear, practical docs rather than tribal
  knowledge.

**What Yoe-NG leaves behind:**

- x86-centric assumptions.
- pacman (using apk instead).
- The expectation of interactive manual system administration.
- Lack of reproducibility guarantees.

**Key differences:**

|                   | Arch                      | Yoe-NG                          |
| ----------------- | ------------------------- | ------------------------------- |
| Target            | Desktop/server, x86-first | Embedded, multi-arch            |
| Package manager   | pacman                    | apk                             |
| Package format    | tar.zst + .PKGINFO        | apk (tar.gz + .PKGINFO)         |
| Build definitions | PKGBUILD (bash)           | Starlark units                  |
| Reproducibility   | Not a goal                | Content-addressed builds        |
| Image assembly    | Manual (pacstrap)         | Automated (`yoe build <image>`) |
| Administration    | Interactive (hands-on)    | Declarative (config-driven)     |

**When to use Arch instead:** when you're building a desktop or server system
for personal use and value having full manual control. Arch's philosophy works
well for power users on general-purpose hardware.

## vs. NixOS / Nix

Nix is the most intellectually ambitious of the systems Yoe-NG draws from. Its
ideas about reproducibility and declarative configuration are adopted wholesale;
its implementation complexity is not.

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
| Config language | Nix (custom functional language)     | Starlark (Python-like)                     |
| Store model     | Content-addressed `/nix/store` paths | Standard FHS with apk                      |
| Closure size    | Often 1GB+ for simple systems        | Target single-digit MB base                |
| Target          | Desktop, server, CI                  | Embedded hardware                          |
| BSP support     | Minimal                              | Per-board machine definitions              |
| Package manager | Nix                                  | apk                                        |
| Reproducibility | Bit-for-bit (aspirational)           | Content-addressed, functionally equivalent |
| Rollback        | Via Nix generations                  | Via A/B partitions or apk                  |
| Learning curve  | Steep (must learn Nix language)      | Shallow (Starlark, Python-like)            |

**Caching comparison:** Nix's binary cache (Cachix, or self-hosted with
`nix-serve`) is conceptually similar to Yoe-NG's remote cache — both store
content-addressed build outputs in S3-compatible storage. The key differences:
Nix caches _closures_ (a package plus all its transitive runtime dependencies),
which can be very large. Yoe-NG caches individual `.apk` packages, which are
smaller and more granular. Nix's content addressing is based on the full
derivation hash (all inputs); Yoe-NG uses a similar scheme but at unit
granularity rather than Nix's per-output granularity.

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
  dependency graph before generating any build files. `yoe build` does the same:
  resolve the entire unit DAG, check for errors, then build. No partial builds
  from graph errors discovered mid-way.
- **Config propagation** — GN's `public_configs` automatically apply compiler
  flags to anything that depends on a target. Yoe-NG propagates machine-level
  settings (arch flags, optimization, kernel headers) through the unit graph.
- **Build introspection** — GN provides `gn desc` (what does this target do?)
  and `gn refs` (what depends on this?). Yoe-NG provides `yoe desc`, `yoe refs`,
  and `yoe graph` for the same purpose.
- **Label-based references** — GN uses `//path/to:target` for unambiguous target
  identification. Yoe-NG uses a similar scheme for composable unit references
  across repositories.

**What Yoe-NG leaves behind:**

- Ninja file generation — Yoe-NG's unit builds are coarse-grained enough that
  `yoe` orchestrates directly.
- GN's custom scripting language — Starlark serves the same purpose for Yoe-NG.
- C/C++ build model specifics — GN is deeply tied to source-file-level
  dependency tracking, which isn't relevant for unit-level builds.

**Key differences:**

|                        | GN                      | Yoe-NG                              |
| ---------------------- | ----------------------- | ----------------------------------- |
| Purpose                | C/C++ meta-build system | Embedded Linux distribution builder |
| Output                 | Ninja build files       | `.apk` packages and disk images     |
| Config language        | GN (custom)             | Starlark (Python-like)              |
| Dependency granularity | Source file / target    | Unit (package)                      |
| Build execution        | Ninja                   | `yoe` directly                      |
| Introspection          | `gn desc`, `gn refs`    | `yoe desc`, `yoe refs`, `yoe graph` |

GN is not an alternative to Yoe-NG — they solve different problems. But GN's
approach to graph resolution, config propagation, and introspection are
well-proven patterns that Yoe-NG applies to the embedded Linux domain.

## Value Proposition and Strategic Positioning

### The Core Thesis

Yocto's model of wrapping every dependency in a unit made sense when C/C++ was
the only game in town and there was no dependency management beyond "whatever
headers are on the system." Modern languages have solved this:

- **Go**: `go.sum` is a cryptographic lock file. Builds are already
  reproducible.
- **Rust**: `Cargo.lock` pins every transitive dependency.
- **Zig**: Hash-pinned dependencies.
- **Node/Python**: Lock files are standard practice.

Yocto's response is to re-declare every dependency the language toolchain
already knows about — `SRC_URI` with checksums for each crate,
`LIC_FILES_CHKSUM` for each module. This is busywork that duplicates what
`Cargo.lock` and `go.sum` already guarantee.

Yoe-NG's position: **let the language package manager do its job.** A Go unit
should declare _what_ to build, not _how to resolve every transitive
dependency_. Content-addressed caching hashes the output — if inputs haven't
changed, the output is the same. You get reproducibility without micromanaging
the build.

### Where Yoe-NG Cannot Compete (Yet)

Be honest about the gaps:

**Vendor BSP support is Yocto's real moat.** Every major SoC vendor (NXP, TI,
Qualcomm, Intel, Renesas, MediaTek) ships Yocto BSP layers and supports them.
This is not a technology problem — it's an ecosystem problem that Linux
Foundation backing solves. No amount of technical superiority overcomes "the
silicon vendor gives us a Yocto BSP and supports it."

**Package count.** Yocto has thousands of units, Buildroot has ~2800. Yoe-NG has
a handful. Need curl, dbus, python3, or ffmpeg? You have to write the unit.

**Configuration UX.** Buildroot's `make menuconfig` is a killer feature —
visual, discoverable, searchable. You can explore what's available without
reading unit files. Yoe-NG requires editing Starlark by hand.

**Documentation and community.** Yocto has comprehensive manuals, Bootlin
training materials, and years of mailing list archives. Buildroot has a
well-maintained manual and active list. Problems are googleable. Yoe-NG has
design docs and a small team.

**Legal compliance tooling.** Yocto's `do_populate_lic` and Buildroot's
`make legal-info` generate license manifests and source archives. This is
required for shipping products in many industries. Yoe-NG has nothing here yet.

**Proven production track record.** Thousands of products ship with Yocto.
Buildroot runs on millions of devices. Yoe-NG is a prototype.

### Where Yoe-NG Can Win

**Target audience:** Teams building Go/Rust/Zig services for embedded Linux —
edge computing, IoT gateways, network appliances. Teams where the application
_is_ the product, not the base OS. Teams that want "Alpine + my app on custom
hardware" not "custom Linux distro with 200 hand-tuned units."

These teams currently use Buildroot, hack together Docker-based builds, or
cross-compile manually. They would never adopt Yocto because the overhead is
absurd for their use case.

**First-class modern language support.** Go/Rust/Zig unit classes should be
trivial to use. The build system should get out of the way and let `go build`,
`cargo build`, and `zig build` do their jobs. This is where Yocto is most out of
touch.

**Custom hardware without desktop distro limitations.** Desktop distros (Debian,
Fedora, Alpine) have great package management but no story for custom kernels,
device trees, bootloaders, board-specific firmware, or flash/deploy workflows.
This is the entire reason Yocto and Buildroot exist. Yoe-NG should provide BSP
tooling (machine definitions, kernel units, `yoe flash`, `yoe run`) that is
simpler than Yocto's but more capable than anything desktop distros offer.

**Incremental builds and shared caching.** Buildroot rebuilds everything from
scratch. Yocto's sstate is powerful but complex to set up. Yoe-NG's
content-addressed `.apk` cache in S3-compatible storage is conceptually simpler:
push packages to a bucket, pull them on other machines. CI builds once,
developers reuse the output.

**AI-assisted unit generation.** If an AI can generate a working Starlark unit
from a project URL faster than porting a Yocto unit, the small package count
stops mattering. Starlark is far more tractable for AI than BitBake's metadata
format.

### The Alpine Linux Precedent

Alpine didn't supplant Debian — it became the default for containers because it
was radically smaller and simpler for that specific use case. Yoe-NG doesn't
need to replace Yocto for automotive or aerospace. It needs to be the obvious
choice for a specific class of embedded product where Yocto is overkill and
Buildroot is too limited.

### What to Focus On

1. **Modern language unit classes** — Go, Rust, Zig should be first-class, not
   afterthoughts. These are the differentiator. A Go developer should go from "I
   have a binary" to "I have a bootable image on custom hardware" in minutes.

2. **BSP tooling** — machine definitions, kernel/bootloader units, `yoe flash`,
   `yoe run`. This is what desktop distros lack and what justifies Yoe-NG's
   existence as a build system rather than just another distro.

3. **Shared build cache** — the S3-backed package cache is a major advantage
   over Buildroot. Make it trivial to set up so teams see the value immediately.

4. **AI unit generation** — lean into the AI-native angle. If generating a new
   unit is a conversation rather than a manual porting exercise, the package
   count gap closes fast.

5. **Board support** — start with popular, accessible boards (Raspberry Pi,
   BeagleBone, common QEMU targets). Every board that works out of the box is a
   potential user who doesn't need Yocto.

6. **Don't chase Yocto's tail** — resist the urge to add Yocto-like features
   (task-level DAGs, unit splitting, bbappend equivalents) to win over Yocto
   users. Instead, make the simple path so good that teams choose Yoe-NG because
   it fits their workflow, not because it replicates Yocto's.

## Summary Matrix

| Feature                 | Yocto    | Buildroot | Alpine   | Arch     | NixOS   | **Yoe-NG** |
| ----------------------- | -------- | --------- | -------- | -------- | ------- | ---------- |
| Embedded focus          | Yes      | Yes       | Partial  | No       | No      | **Yes**    |
| Simple config           | No       | Moderate  | Moderate | Yes      | No      | **Yes**    |
| Native builds           | No       | No        | Yes      | Yes      | Yes     | **Yes**    |
| On-device packages      | Optional | No        | Yes      | Yes      | Yes     | **Yes**    |
| Content-addressed cache | Partial  | No        | No       | No       | Yes     | **Yes**    |
| Remote shared cache     | Complex  | No        | No       | No       | Yes     | **Yes**    |
| Pre-built package cache | No       | No        | Yes      | Yes      | Yes     | **Yes**    |
| Declarative images      | Yes      | Partial   | No       | No       | Yes     | **Yes**    |
| Multi-image support     | Yes      | No        | No       | No       | Yes     | **Yes**    |
| Image inheritance       | Partial  | No        | No       | No       | Yes     | **Yes**    |
| Custom BSP support      | Yes      | Yes       | No       | No       | Minimal | **Yes**    |
| Incremental updates     | Complex  | No        | Yes      | Yes      | Yes     | **Yes**    |
| Hermetic builds         | Partial  | No        | No       | No       | Yes     | **Yes**    |
| Fast package ops        | N/A      | N/A       | Yes      | Moderate | Slow    | **Yes**    |
