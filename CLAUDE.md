# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

Yoe-NG is a next-generation embedded Linux distribution builder — a simpler
alternative to Yocto. Currently in the **design/documentation phase** with no
source code yet. The repository contains architectural specifications across
several markdown files.

Core design: Go CLI (`yoe`) + TOML metadata + apk packages + bubblewrap
isolation. Native builds only (no cross-compilation). Base system: glibc +
busybox + systemd.

## Repository Structure

- `README.md` — project philosophy, design goals, comparisons overview
- `yoe-tool.md` — `yoe` CLI command reference (init, build, image, flash, etc.)
- `metadata-format.md` — TOML spec for recipes, machines, images, partitions
- `build-environment.md` — three-tier build isolation architecture (bootstrap,
  build root, per-recipe sandbox)
- `comparisons.md` — detailed comparison with Yocto, Buildroot, Alpine, Arch,
  NixOS, GN
- `envsetup.sh` — shell functions (source it, don't execute)

## Commands

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

## Key Design Decisions

- **TOML** for all metadata (not YAML/JSON)
- **apk** package manager (same as Alpine, but with glibc)
- **bubblewrap** for build isolation (not Docker) — 1ms overhead, unprivileged,
  no daemon
- **User namespaces** for pseudo-root (not fakeroot/pseudo) — stateless, works
  with static binaries
- **Native builds only** — no cross-compilation; modern ARM/RISC-V hardware
  makes this feasible
- **Language-native package managers** (Go modules, Cargo, npm, pip) instead of
  reimplementing dependency resolution
- **Label-based references** inspired by Google GN (e.g.,
  `github.com/yoe/recipes-core//openssh@v1.2.0`)
- **Two-phase build**: resolve DAG then execute (inspired by GN's
  generate-then-build)
- **Content-addressed caching**: input hash determines output, enabling remote
  cache sharing
