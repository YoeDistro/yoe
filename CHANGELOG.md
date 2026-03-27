# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- **Busybox init** — images use busybox `/sbin/init` with a minimal `/etc/inittab`
  instead of `init=/bin/sh`. Shell respawns on exit, clean shutdown via
  `poweroff`.

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
