# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- **Per-unit sysroots** — each unit's build sysroot is assembled from only its
  transitive `deps`, not every previously built unit. Fixes busybox symlinks
  shadowing container tools (e.g., musl-linked `expr` breaking autoconf).
- **Run from TUI** — press `r` on an image unit to launch it in QEMU.

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
