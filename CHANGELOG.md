# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.0] - 2026-03-31

**BASE-IMAGE boots on RPI4**

- **Tasks replace build steps** — `build = [...]` replaced by `tasks = [...]`
  with named build phases. Each task has `run` (shell string), `fn` (Starlark
  function), or `steps` (mixed list). Classes (autotools, cmake, go) are now
  pure Starlark.
- **`run()` builtin** — Starlark functions can execute shell commands directly
  during builds. Errors show `.star` file and line number, not generated shell.
  `run(cmd, check=False)` returns exit code/stdout/stderr for conditional logic.
  `run(cmd, privileged=True)` runs directly in the container as root for
  operations like losetup/mount that bwrap can't do.
- **Unit scope** — units declare `scope = "machine"`, `"noarch"`, or `"arch"`
  (default). Machine-scoped units (kernels, images) build per-machine. Build
  directories are flat: `build/<name>.<scope>/`. Repo is flat with scope in
  filenames: `repo/<name>-<ver>-r0.<scope>.apk`.
- **Machine-portable images** — images no longer hard-code machine-specific
  packages or partitions. `MACHINE_CONFIG` and `PROVIDES` inject machine
  hardware specifics automatically. `base-image` works across QEMU x86, QEMU
  arm64, and Raspberry Pi without changes.
- **`PROVIDES` virtual packages** — units and kernels declare `provides` to
  fulfill virtual names. `provides = "linux"` on `linux-rpi4` means images that
  list `"linux"` get the RPi kernel when building for `raspberrypi4`.
- **Image assembly in Starlark** — disk image creation moved from Go to
  `classes/image.star` using `run()`. Fully readable, customizable, forkable.
- **Raspberry Pi BSP module** (`units-rpi`) — machine definitions, kernel fork
  units, GPU firmware, and boot config for Raspberry Pi 4 and 5.
- **Runtime dependency resolution** — image assembly now resolves transitive
  runtime dependencies automatically. `RUNTIME_DEPS` predeclared variable
  available after unit evaluation. Three-phase loader: machines → units →
  images.
- **Layers renamed to modules** — `layer()` → `module()`, `LAYER.star` →
  `MODULE.star`, `yoe layer` → `yoe module`, `layers/` → `modules/`. Aligns
  terminology with Go modules model used for dependency resolution.

## [0.4.0] - 2026-03-31

**ARM BUILDS ON X86 NOW WORK**

- **TUI global notifications** — the TUI now shows a yellow banner for
  background operations like container image rebuilds. Previously these events
  were only visible in build log files.
- **cmake added to build container** — cmake is now available as a bootstrap
  tool in the container (version bump to 14), enabling units that use the cmake
  build system.
- **xz switched to cmake** — the xz unit now uses the cmake class instead of
  autotools with gettext workarounds, simplifying the build definition.
- **TUI reloads .star files before each build** — editing unit definitions or
  classes no longer requires restarting the TUI. The project is re-evaluated
  from Starlark on each build, picking up any changes to build steps, deps, or
  configuration.
- **Fix xz autoreconf failure** — xz's `configure.ac` uses `AM_GNU_GETTEXT`
  macros which require gettext's m4 files. The xz unit now provides stub m4
  macros and skips `autopoint`, allowing `autoreconf` to succeed without gettext
  installed in the container.
- **Cross-architecture builds** — build arm64 and riscv64 images on x86_64 hosts
  using QEMU user-mode emulation. Target arch is resolved from the machine
  definition. Run `yoe container binfmt` for one-time setup, then
  `yoe build base-image --machine qemu-arm64` works transparently.
- **Arch-aware build directories** — build output is now stored under
  `build/<arch>/<unit>/` and APK repos under `build/repo/<arch>/`, supporting
  multi-arch builds in the same project. **Note:** existing build caches under
  `build/<unit>/` will need to be rebuilt (`yoe clean --all`).
- **`yoe container binfmt`** — new command to register QEMU user-mode emulation
  for cross-architecture container builds. Shows what it will do and prompts for
  confirmation.
- **Multi-arch QEMU** — `yoe run` now auto-detects cross-architecture execution
  and uses software emulation (`-cpu max`) instead of KVM. Container includes
  `qemu-system-aarch64` and `qemu-system-riscv64`.
- **TUI setup menu** — press `s` to open a setup view for selecting the target
  machine. Shows available machines with their architecture and highlights the
  current selection. Designed to accommodate future setup options.

## [0.3.4] - 2026-03-30

- **Build lock files** — a PID-based `.lock` file is written during builds so
  other `yoe` instances can detect in-progress work instead of marking active
  builds as failed. Builds are skipped if another process is already building
  the same unit.
- **`yoe clean --locks`** — removes stale lock files left behind by crashed or
  killed builds.
- **TUI edit for cached layers** — pressing `e` on a unit now also searches the
  layer cache, so editing works for units from layers cloned via
  `yoe layer sync`.

## [0.3.3] - 2026-03-30

- **HTTPS layer URLs** — `yoe init` now uses HTTPS URLs for the units-core layer
  instead of SSH, removing the need for SSH key setup to get started.

## [0.3.2] - 2026-03-30

- **TUI scrolling** — both the unit list and detail log views are now
  scrollable. The unit list shows `↑`/`↓` overflow indicators when there are
  more units than fit on screen. The detail view supports `j`/`k`,
  `PgUp`/`PgDn`, `g`/`G` navigation through the full build output and log, with
  auto-follow during active builds.
- **Auto-sync layers** — `yoe build` and other commands that load the project
  now automatically clone missing layers on first use, matching the lazy
  container-build pattern. Existing cached layers are not fetched/updated, so
  there is no added latency on subsequent runs. Explicit `yoe layer sync` is
  still available to update layers.
- **TUI confirmation prompts** — quitting (`q`/`ctrl+c`) and cancelling a build
  (`x`) now prompt for confirmation when builds are active, preventing
  accidental loss of in-progress builds. Declining a prompt clears the message
  cleanly.
- **Fix build cancellation not stopping containers** — cancelling a build (via
  TUI quit or `ctrl+c` on the CLI) now explicitly stops the Docker container
  (`docker stop`) instead of only killing the CLI client, which left containers
  running in the background.
- **Fix stale cache after cancelled builds** — the cache marker is now removed
  before building so a cancelled or failed rebuild no longer appears cached from
  a previous successful build.

## [0.3.1] - 2026-03-30

**ALL UNITS ARE NOW BUILDING**

- **Per-unit sysroots** — each unit's build sysroot is assembled from only its
  transitive `deps`, not every previously built unit. Fixes busybox symlinks
  shadowing container tools (e.g., musl-linked `expr` breaking autoconf).
- **Run from TUI** — press `r` on an image unit to launch it in QEMU.
- **Log writer plumbing** — container stdout/stderr in image assembly and source
  fetch/prepare output now route through the build log writer instead of
  os.Stdout. Fixes TUI alt-screen corruption during background builds.
- **Autotools maintainer-mode override** — `make` invocations pass
  `ACLOCAL=true AUTOCONF=true AUTOMAKE=true AUTOHEADER=true MAKEINFO=true` to
  prevent re-running versioned autotools (e.g., `aclocal-1.16`) that aren't in
  the container. Fixes gawk and similar packages.
- **rcS init script** — `base-files` now includes `/etc/init.d/rcS` which runs
  all `/etc/init.d/S*` scripts at boot.
- **network-config unit** — new unit that configures a network interface via an
  init script.
- **Build failure context** — when a unit fails, the output now lists all
  downstream units blocked by the failure. The TUI shows cached units in blue
  and displays the full build queue (waiting/cached) before work begins.
- **dev-image** — added `kmod` and `util-linux` to the development image.
- **Image rootfs dep fix** — image assembly now follows only `runtime_deps` when
  resolving packages, not build-time `deps`. Fixes build-only packages (e.g.,
  gettext via xz) being installed into the rootfs and overflowing the partition.

## [0.3.0] - 2026-03-30

**THIS RELEASE DOES NOT WORK** - this release is only to capture rename and TUI
updates. Wait for a future one to do any work.

**BREAKING CHANGE** - due to rename, recommend deleting any external projects
and starting over.

- **Terminology rename** — "recipe" is now "unit" and "package" is now
  "artifact" throughout the codebase. The Starlark `package()` function is now
  `unit()`, the image field `packages` is now `artifacts`, and the `recipes/`
  directory in layers is now `units/`. The `recipes-core` layer is now
  `units-core`. The Go `internal/packaging` package is now `internal/artifact`.
- **`yoe log`** — view build logs from the command line. Shows the most recent
  build log by default, or a specific unit's log with `yoe log <unit>`. Use `-e`
  to open the log in `$EDITOR`.
- **`yoe diagnose`** — launch Claude Code with the `/diagnose` skill to analyze
  a build failure. Uses the most recent build log by default, or a specific
  unit's log with `yoe diagnose <unit>`.
- **TUI rewrite** — `yoe` with no args launches an interactive unit list with
  inline build status (cached/waiting/building/failed). Builds run in-process
  via `build.BuildUnits()` with real-time status events — dependencies show as
  yellow "waiting", then flash green as they build. Features: background builds
  (`b`/`B`), edit unit in `$EDITOR` (`e`), view build log (`l`), diagnose with
  Claude (`d`), add unit with Claude (`a`), clean with confirmation (`c`/`C`),
  search/filter (`/`), and a split detail view showing executor output and build
  log tail. The `yoe tui` subcommand has been removed.
- **Build events** — `build.Options.OnEvent` callback notifies callers (e.g.,
  the TUI) as each unit transitions through cached/building/done/failed states.

## [0.2.10] - 2026-03-30

- **`yoe container shell`** — interactive bash shell inside the build container
  with bwrap sandbox, sysroot mounts, and the same environment variables recipes
  see during builds. Useful for debugging build failures and sandbox issues.

## [0.2.9] - 2026-03-30

- **Bash for build commands** — switched build shell from busybox sh to bash.
  Avoids autoconf compatibility issues (e.g., `AS_LINENO_PREPARE` infinite loop)
  and matches what upstream build scripts expect. Removed per-recipe bash
  workaround from util-linux.
- **User account API** — new `classes/users.star` provides `user()` and
  `users_commands()` functions for defining user accounts in Starlark.
  `base-files` is now a callable `base_files()` function that accepts a `users`
  parameter — image recipes can override it to add users (e.g., dev-image adds a
  `user` account with password `password`).

## [0.2.8] - 2026-03-30

- **meson build system support** — added samurai (ninja-compatible build tool),
  meson, and kmod recipes. Container updated to v11 with python3 and
  py3-setuptools for meson. Build environment now sets `PYTHONPATH` to the
  sysroot so Python packages installed by recipes are discoverable.
- **Container versioning note** — CLAUDE.md now documents that both
  `Dockerfile.build` and `internal/container.go` must be bumped together.
- **gettext recipe** — builds GNU gettext from source as a recipe instead of
  relying on the container. Provides `autopoint` needed by packages like xz that
  use gettext macros in their autotools build.
- **Sysroot binaries on PATH** — `/build/sysroot/usr/bin` is now prepended to
  `PATH` during builds, so executables from dependency recipes are discoverable.
- Autotools class respects explicit `build` steps — no longer prepends default
  autoreconf/configure when a recipe provides its own build commands.
- **Claude Code plugin** — added `.claude/` plugin with AI skills for recipe
  development: `diagnose` (iterative build failure analysis), `new-recipe`
  (generate recipes from URLs/descriptions), `update-recipe` (version bumps),
  `audit-recipe` (review against best practices and other distros).
- **`--clean` build flag** — deletes source and destdir before rebuilding.
  `--force` now only skips the cache check without cleaning.
- **`--force`/`--clean` scoped to requested recipes** — dependency recipes still
  use the cache, only explicitly named recipes are force-rebuilt.
- Fixed `YOE_CACHE` help text — was `~/.cache/yoe-ng`, actually defaults to
  `cache/` in the project directory.

## [0.2.7] - 2026-03-27

- **Per-recipe build logs** — build output written to
  `build/<recipe>/build.log`. Console is quiet by default; on error the log path
  is printed. Use `--verbose` / `-v` to stream build output to the console.
- Fixed QEMU machine templates — removed UEFI firmware (`ovmf`/`aavmf`/
  `opensbi`) incompatible with MBR+syslinux boot, fixed root device `vda2` →
  `vda1`.

## [0.2.6] - 2026-03-27

- **base-files recipe** — provides filesystem skeleton: `/etc/passwd` (root with
  blank password), `/etc/inittab` (busybox init + getty), `/boot/extlinux/`
  (boot config), and essential mount point dirs (`/proc`, `/sys`, `/dev`, etc.).
  Moved from hardcoded Go to a recipe so users can customize via overlays.
- Serial console uses `getty` for proper login prompt.

## [0.2.5] - 2026-03-27

### Added

- **musl libc recipe** — copies the musl dynamic linker from the build container
  into the image so dynamically linked packages work at runtime.
- **Automatic package dep resolution** — image assembly now resolves transitive
  build and runtime deps from recipe metadata. e.g., openssh automatically pulls
  in openssl and zlib without listing them in the image recipe.
- **Recipes without source** — recipes with no `source` field (e.g., musl) skip
  source preparation instead of erroring.

### Fixed

- Disable ext4 features (`64bit`, `metadata_csum`, `extent`) incompatible with
  syslinux 6.03 so bootloader can load kernel from any partition size.
- Image package dep resolution walks both `deps` and `runtime_deps` so shared
  libraries are included.
- OpenSSL recipe uses `--libdir=lib` so libraries install to `/usr/lib` instead
  of `/usr/lib64` — fixes "Error loading shared library libcrypto.so.3".
- Inittab no longer tries to mount `/dev` (already mounted by kernel via
  `devtmpfs.mount=1`).
- Skip `TestBuildRecipes_WithDeps` in CI — GitHub Actions runners don't support
  user namespaces inside Docker.
- Most stuff in `dev-image` now works.

## [0.2.4] - 2026-03-27

- update BL config

## [0.2.3] - 2026-03-27

### Changed

- **Container as build worker** — `yoe` CLI always runs on the host. The
  container is now a stateless build worker invoked only for commands that need
  container tools (gcc, bwrap, mkfs, etc.). Eliminates container startup
  overhead for read-only commands (`config`, `desc`, `refs`, `graph`, `clean`).
- **File ownership** — build output uses `--user uid:gid` so files created by
  the container are owned by the host user, not root.
- **QEMU host-first** — `yoe run` tries host `qemu-system-*` first, falls back
  to the container if not found.
- **`--force` scoped to requested recipes** — `--force` and `--clean` only
  force-rebuild the explicitly requested recipes; dependencies still use the
  cache for incremental builds.
- **Busybox init** — images use busybox `/sbin/init` with a minimal
  `/etc/inittab` instead of `init=/bin/sh`. Shell respawns on exit, clean
  shutdown via `poweroff`.

### Fixed

- Shell quoting in bwrap sandbox commands — semicolons in env exports no longer
  split the command at the outer shell level.
- Package installation in image assembly — always extracts `.apk` files via
  `tar` instead of gating on `apk` binary availability.
- Rootfs mount points (`/proc`, `/sys`, `/dev`, `/tmp`, `/run`) now included in
  disk images via `.keep` placeholder files.
- `devtmpfs.mount=1` added to kernel cmdline so `/dev` is populated before init.

### Removed

- `YOE_IN_CONTAINER` environment variable — no longer needed.
- `ExecInContainer` / `InContainer` / `HasBwrap` APIs — replaced by
  `RunInContainer`.
- Container re-exec pattern — the yoe binary is no longer bind-mounted into the
  container.

## [0.2.2] - 2026-03-27

### Added

- **Layer `path` field** — layers can live in a subdirectory of a repo via
  `path = "layers/recipes-core"`. Layer name derived from path's last component.
- **Project-local cache** — source and layer caches default to `cache/` in the
  project directory instead of `~/.cache/yoe-ng/`
- **`.gitignore` in `yoe init`** — new projects get a `.gitignore` with `/build`
  and `/cache`
- **Autotools `autoreconf`** — autotools class auto-runs `autoreconf -fi` when
  `./configure` is missing (common with git sources)
- SSH URL support for source fetching (`git@host:user/repo.git`)
- **Design: per-recipe tasks and containers** — planned support for named
  `task()` build steps with optional per-task Docker container images. Container
  resolves: task → package → bwrap. See
  `docs/superpowers/plans/per-recipe-containers.md`.

### Changed

- Default layer in `yoe init` uses SSH URL
  (`git@github.com:YoeDistro/yoe-ng.git`) with `path = "layers/recipes-core"`
- Container no longer mounts a separate cache volume — cache/ is accessible
  through the project mount
- Container runs with `--privileged` (needed for losetup/mount during disk image
  creation and /dev/kvm for QEMU)

## [0.2.1] - 2026-03-27

### Added

- **Dev-image with 10+ packages** — new `dev-image` builds end-to-end with
  sysroot, including essential libraries (openssl, ncurses, readline, libffi,
  expat, xz), networking (curl, openssh), and debug tools (strace, vim)
- **Remote layer fetching** — `yoe layer sync` clones/fetches layers from Git
- **Sysroot + image deps in DAG** — build sysroot and image dependencies
  resolved as part of the dependency graph
- **`yoe_sloc`** — source lines of code counter using `scc`

### Fixed

- Correct partition size for `losetup`, ensure sysroot dir exists
- Recipe fixes for end-to-end dev-image builds

### Changed

- Moved design docs into `docs/` directory
- Expanded build-environment and comparisons documentation

## [0.2.0] - 2026-03-26

### Added

- **Bootable QEMU x86_64 image** — end-to-end flow from recipes to a partitioned
  disk image that boots to a Linux kernel with busybox
- **Starlark `load()` support** — class imports and `@layer//path` label-based
  references across layers, `//` resolves to layer root when inside a layer
- **Recursive recipe discovery** — `recipes/**/*.star` directory traversal
- **`recipes-core` layer** — autotools/cmake/go/image classes, busybox/zlib/
  syslinux/linux recipes, base-image, qemu-x86_64 machine
- **APKINDEX generation** — `APKINDEX.tar.gz` for apk dependency resolution
- **Bootstrap framework** — `yoe bootstrap stage0/stage1/status`
- **Container auto-enter** — host `yoe` binary bind-mounted into container,
  Dockerfile embedded in binary, versioned image tags

### Fixed

- Build busybox as static binary (no shared lib dependency on rootfs)
- APKINDEX uses SHA1 base64 as required by apk
- Handle git sources in workspace (tag upstream without re-init)
- bwrap sandbox inside Docker with `--security-opt seccomp=unconfined`
- Mount git root for layer resolution

### Changed

- Prefer git sources with shallow clone over tarballs
- Move build commands to `envsetup.sh` (`yoe_build`, `yoe_test`)

## [0.1.0] - 2026-03-26

Initial release of yoe-ng — a next-generation embedded Linux distribution
builder.

### Added

- **CLI foundation** — `yoe init`, `yoe config show`, `yoe clean`, `yoe layer`
  commands with stdlib switch/case dispatch (no framework)
- **Starlark evaluation engine** — recipe and configuration evaluation using
  go.starlark.net with built-in functions (`project()`, `machine()`,
  `package()`, `image()`, `layer_info()`, etc.)
- **Dependency resolution** — DAG construction, Kahn's algorithm topological
  sort with cycle detection, `yoe desc`, `yoe refs`, `yoe graph`
- **Content-addressed hashing** — SHA256 cache keys from recipe + source +
  patches + dep hashes + architecture
- **Source management** — `yoe source fetch/list/verify/clean` with
  content-addressed cache and patch application
- **Build execution** — `yoe build` with bubblewrap per-recipe sandboxing,
  automatic container isolation via Docker/Podman
- **Package creation** — APK package creation, `yoe repo` commands, local
  repository management
- **Image assembly** — rootfs construction, overlay application, disk image
  generation with syslinux MBR + extlinux
- **Device interaction** — `yoe flash` with safety checks, `yoe run` for QEMU
  with KVM
- **Interactive TUI** — Bubble Tea interface for browsing recipes and machines
- **Developer workflow** — `yoe dev extract/diff/status` for source modification
- **Custom commands** — extensible CLI via `commands/*.star`
- **Patch support** — per-recipe patch files applied as git commits
