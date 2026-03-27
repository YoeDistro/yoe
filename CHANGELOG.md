# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Layer `path` field** — layers can live in a subdirectory of a repo via
  `path = "layers/recipes-core"`. Layer name derived from path's last component.
- **Project-local cache** — source and layer caches default to `cache/` in the
  project directory instead of `~/.cache/yoe-ng/`
- **`.gitignore` in `yoe init`** — new projects get a `.gitignore` with `/build`
  and `/cache`
- **Build sysroot** — after each package builds, its output is installed into
  `build/sysroot/` so subsequent recipes can find deps' headers/libraries via
  `CFLAGS`, `LDFLAGS`, and `PKG_CONFIG_PATH`
- **Image deps in DAG** — image recipes' `packages` list is treated as
  dependencies so `yoe build dev-image` automatically builds all required
  packages first
- **Dev-image with 10+ packages** — busybox, linux, syslinux, ncurses, strace,
  vim, zlib, openssl, curl, openssh — all built from git sources
- **Remote layer fetching** — `yoe layer sync` clones/fetches layers declared in
  `PROJECT.star` into the local cache
- **Autotools `autoreconf`** — autotools class auto-runs `autoreconf -fi` when
  `./configure` is missing (common with git sources)
- SSH URL support for source fetching (`git@host:user/repo.git`)
- `yoe layer` runs on the host (no container required)
- `yoe_sloc` — source lines of code counter using `scc`

### Changed

- Default layer in `yoe init` uses SSH URL
  (`git@github.com:YoeDistro/yoe-ng.git`) with `path = "layers/recipes-core"`
- Container no longer mounts a separate cache volume — cache/ is accessible
  through the project mount
- Container runs with `--privileged` (needed for losetup/mount during disk image
  creation and /dev/kvm for QEMU)
- Moved design docs into `docs/` directory

### Fixed

- Correct partition size for `losetup` (match ext4 fs to partition boundaries)
- Recipe fixes: ncurses v6.4, strace v6.9 with `./bootstrap`, vim static
  ncurses, curl `--without-libpsl`, openssh `--without-openssl-header-check`
- ext4 partition size matches filesystem (add 1MB for MBR overhead)
- Attach TTY to container when stdin is a terminal (needed for `yoe run`)

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
