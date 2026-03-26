# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
  sort with cycle detection, transitive dep/rdep queries, `yoe desc`,
  `yoe refs`, `yoe graph` (text and DOT output)
- **Content-addressed hashing** — SHA256 cache keys from recipe + source +
  patches + dep hashes + architecture
- **Layer management** — `yoe layer list`, `LAYER.star` support, transitive
  layer dependencies
- **Source management** — `yoe source fetch/list/verify/clean` with
  content-addressed cache, checksum verification, and patch application
- **Build execution** — `yoe build` with bubblewrap per-recipe sandboxing, build
  step execution, automatic container isolation via Docker/Podman
- **Container isolation** — automatic re-execution inside Alpine-based container
  with embedded Dockerfile, versioned image management, Docker and Podman
  support
- **Package creation** — APK package creation, `yoe repo` commands, local
  repository management
- **Image assembly** — rootfs construction via apk, overlay application, disk
  image generation with systemd-repart
- **Device interaction** — `yoe flash` for writing images to devices, `yoe run`
  for QEMU with KVM acceleration
- **Interactive TUI** — Bubble Tea interface for browsing recipes, viewing
  dependencies, and monitoring builds
- **Developer workflow** — `yoe dev` for source modification workflow,
  extensible custom commands via Starlark
- **Patch support** — per-recipe patch directories with ordered application
- **CI/CD** — GitHub Actions workflows for testing, building, and
  GoReleaser-based releases
