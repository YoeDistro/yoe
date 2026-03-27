# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

Yoe-NG is a next-generation embedded Linux distribution builder — a simpler
alternative to Yocto. The project has a working Go CLI (`yoe`) with Starlark
recipe evaluation, container-based builds, DAG resolution, apk packaging,
bubblewrap sandboxing, image generation, and a TUI. A `recipes-core` layer
provides initial classes (autotools, cmake, go, image) and recipes.

Core design: Go CLI (`yoe`) + Starlark recipes/config + apk packages +
bubblewrap isolation. Native builds only (no cross-compilation). Base system:
glibc + busybox + systemd.

## CRITICAL: Container-Only Build Policy

**All build operations run inside the Docker/Podman container. The host provides
ONLY the `yoe` binary and Docker. Nothing else from the host should leak into
builds.**

- The host has NO build tools (no gcc, no bwrap, no apk, no make)
- The container (`yoe-ng:<version>`) provides ALL build tools
- `yoe` on the host auto-enters the container for any command that needs tools
- Only `yoe init`, `yoe version`, `yoe tui`, and `yoe container` run on the host
- Everything else (`build`, `config`, `source`, `desc`, `graph`, etc.) runs in
  the container
- The container runs with `--security-opt seccomp=unconfined` so bubblewrap can
  create user namespaces inside Docker
- The host `yoe` binary is bind-mounted into the container
  (`-v yoe:/usr/local/bin/yoe:ro`), not baked into the image
- Never assume any tool is available on the host — if it's needed, it goes in
  `containers/Dockerfile.build`

This is non-negotiable. The entire point is that developers need only Docker and
the `yoe` binary. No host dependencies beyond that.

## Repository Structure

### Documentation

- `README.md` — project philosophy, design goals, comparisons overview
- `docs/yoe-tool.md` — `yoe` CLI command reference (init, build, flash, etc.)
- `docs/metadata-format.md` — Starlark recipe and configuration spec
- `docs/build-environment.md` — three-tier build isolation architecture
  (bootstrap, build root, per-recipe sandbox)
- `docs/build-languages.md` — analysis of Starlark, CUE, Nix, and other
  embeddable languages
- `docs/ai-skills.md` — AI-driven workflows catalog
- `docs/sdk.md` — development environments and pre-built packages
- `docs/comparisons.md` — comparison with Yocto, Buildroot, Alpine, etc.
- `comparisons.md` — comparisons with other build systems
- `CHANGELOG.md` — release changelog

### Source Code

- `cmd/yoe/main.go` — CLI entry point
- `internal/` — core packages:
  - `starlark/` — Starlark engine, loader, builtins, and recipe evaluation
  - `resolve/` — DAG resolution, dependency graphing, content hashing
  - `build/` — build executor and bubblewrap sandbox
  - `bootstrap/` — bootstrap environment setup
  - `packaging/` — apk package creation
  - `image/` — rootfs assembly and disk image generation
  - `source/` — source fetching and workspace management
  - `repo/` — package repository indexing
  - `config/` — project configuration
  - `device/` — flash and QEMU support
  - `tui/` — terminal UI (bubbletea)
  - `clean.go`, `init.go`, `update.go`, `dev.go`, `layer.go`, `container.go`,
    `configcmd.go` — top-level CLI command implementations

### Layers and Recipes

- `layers/recipes-core/` — core layer with `LAYER.star` manifest
  - `classes/` — build pattern functions (autotools, cmake, go, image)
  - `recipes/` — package recipes (base, bootloaders, libs)
  - `machines/` — machine configs (qemu-x86_64)
  - `images/` — image definitions (base-image)

### Build Infrastructure

> > > > > > > origin/main

- `envsetup.sh` — shell functions (source it, don't execute)
- `containers/Dockerfile.build` — the build container (Tier 0)
- `scripts/` — helper scripts
- `testdata/` — test fixtures

## Commands

### Building yoe

```bash
source envsetup.sh
yoe_build        # builds static binary (CGO_ENABLED=0 for Alpine container)
yoe_test         # run all tests
```

CGO_ENABLED=0 is required because `net/http` pulls in cgo's DNS resolver by
default, producing a dynamically linked binary that won't run inside the Alpine
(musl) container. `yoe_build` handles this automatically.

### Formatting (markdown)

```bash
source envsetup.sh
yoe_format        # format all markdown with prettier
yoe_format_check  # check formatting compliance
```

### CI

The GitHub Actions workflow (`doc-check.yaml`) runs `prettier --check` on all
`**/*.md` files using Node.js 20. Prettier config: `proseWrap: always`
(`.prettierrc`).

## Plans

Implementation plans live in `docs/superpowers/plans/`. After completing work
that corresponds to plan tasks, update the relevant checkboxes (`- [ ]` →
`- [x]`) and phase status in those plans.

## Commit Hygiene

When committing changes, update related documentation and the changelog:

- **CHANGELOG.md** — add an entry describing the change under the appropriate
  section (Added, Changed, Fixed, etc.)
- **Docs** — if the change affects CLI behavior, recipe format, build process,
  or architecture, update the relevant file(s) in `docs/` and/or `README.md`

Do this as part of the same commit, not as a follow-up.

## Key Design Decisions

- **Container-only builds** — host provides only `yoe` + Docker; all tools live
  in the container
- **Starlark** for all recipes and config (Python-like, deterministic,
  sandboxed)
- **Classes as functions** — build patterns (autotools, cmake, image) are
  Starlark functions, not a type system
- **apk** package manager (same as Alpine, but with glibc)
- **bubblewrap** for per-recipe build isolation inside the container — 1ms
  overhead, unprivileged, no daemon
- **User namespaces** for pseudo-root (not fakeroot/pseudo) — stateless, works
  with static binaries
- **Native builds only** — no cross-compilation; modern ARM/RISC-V hardware
  makes this feasible
- **Language-native package managers** (Go modules, Cargo, npm, pip) instead of
  reimplementing dependency resolution
- **Label-based references** inspired by Bazel (e.g.,
  `load("@recipes-core//openssh.star", "openssh")`)
- **Two-phase build**: resolve DAG then execute (inspired by GN's
  generate-then-build)
- **Content-addressed caching**: input hash determines output, enabling remote
  cache sharing
