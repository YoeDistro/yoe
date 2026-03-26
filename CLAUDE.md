# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

Yoe-NG is a next-generation embedded Linux distribution builder — a simpler
alternative to Yocto. Currently in the **design/documentation phase** with no
source code yet. The repository contains architectural specifications across
several markdown files.

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
- The host `yoe` binary is bind-mounted into the container (`-v yoe:/usr/local/bin/yoe:ro`),
  not baked into the image
- Never assume any tool is available on the host — if it's needed, it goes in
  `containers/Dockerfile.build`

This is non-negotiable. The entire point is that developers need only Docker and
the `yoe` binary. No host dependencies beyond that.

## Repository Structure

- `README.md` — project philosophy, design goals, comparisons overview
- `yoe-tool.md` — `yoe` CLI command reference (init, build, flash, etc.)
- `metadata-format.md` — Starlark recipe and configuration spec
- `build-environment.md` — three-tier build isolation architecture (bootstrap,
  build root, per-recipe sandbox)
- `build-languages.md` — analysis of Starlark, CUE, Nix, and other embeddable
  languages
- `envsetup.sh` — shell functions (source it, don't execute)
- `containers/Dockerfile.build` — the build container (Tier 0)

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

## Key Design Decisions

- **Container-only builds** — host provides only `yoe` + Docker; all tools
  live in the container
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
