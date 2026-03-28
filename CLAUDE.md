# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

Yoe-NG is a next-generation embedded Linux distribution builder — a simpler
alternative to Yocto. The project has a working Go CLI (`yoe`) that builds
packages from Starlark recipes inside a Docker container, creates bootable disk
images, and runs them in QEMU. A `recipes-core` layer provides Starlark classes
and recipes for a minimal Linux system (busybox, kernel, openssl, openssh,
etc.).

Core design: Go CLI (`yoe`) + Starlark recipes/config + apk packages +
bubblewrap sandbox inside Docker. Native builds only (no cross-compilation).

## Container as Build Worker

**The `yoe` CLI always runs on the host. The container is a stateless build
worker invoked only when container-provided tools (gcc, bwrap, mkfs, etc.) are
needed.**

- The host runs: CLI dispatch, Starlark evaluation, DAG resolution, source
  fetch, APK packaging, cache management, all query commands
- The container runs: bwrap-sandboxed compilation, image disk tool operations
  (mkfs, sfdisk, bootloader install), Stage 0 bootstrap
- `RunInContainer()` is the single entry point -- called from the build
  executor, image assembly, and bootstrap
- The container runs with `--privileged` for bwrap namespaces and disk tools
- Build output uses `--user uid:gid` so files are owned by the host user
- The container image is built lazily on first build command
- Developers need only Git, Docker/Podman, and the `yoe` binary

## Repository Structure

- `cmd/yoe/main.go` — CLI entry point with command dispatch
- `internal/` — core Go packages (starlark, build, resolve, source, image,
  packaging, repo, device, tui, bootstrap, layer, config)
- `containers/Dockerfile.build` — the build container (Tier 0), embedded in the
  binary via `containers/embed.go`
- `layers/recipes-core/` — base layer with classes, recipes, machines, images
- `testdata/` — test fixtures including e2e-project
- `envsetup.sh` — shell functions (source it, don't execute)
- `docs/` — design documents (README.md, yoe-tool.md, metadata-format.md,
  build-environment.md, build-languages.md, sdk.md, comparisons.md)

### Layer structure

The `recipes-core` layer lives at `layers/recipes-core/` in this repo. Projects
reference it with `path = "layers/recipes-core"`:

```python
layer("git@github.com:YoeDistro/yoe-ng.git",
      ref = "main",
      path = "layers/recipes-core")
```

The `path` field tells yoe the layer's `LAYER.star` is in a subdirectory of the
cloned repo, not at the root.

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

## Key Design Decisions

- **Container-only builds** — host provides only `yoe` + Git + Docker; all tools
  live in the container
- **No installing packages in the container** — if a build fails because a tool
  or library is missing from the container, the fix is to write a recipe that
  builds it from source (and add it as a `deps` entry), not to install it via
  `apk add` in the Dockerfile. This applies to both build tools (makeinfo,
  bison, help2man) and libraries (zlib, ncurses, libffi). The Dockerfile
  provides only the minimal bootstrap toolchain (gcc, binutils, make, etc.);
  everything else is a recipe. For non-essential features (docs, man pages),
  disabling via configure flags is also acceptable.
- **Build sysroot** — after each package builds, its output is installed into
  `build/sysroot/` so subsequent recipes can find deps' headers/libraries
- **Starlark** for all recipes and config (Python-like, deterministic,
  sandboxed)
- **Classes as functions** — build patterns (autotools, cmake, go) are Starlark
  functions in the layer, not Go builtins. Autotools class auto-runs
  `autoreconf` for git sources missing `./configure`.
- **Prefer git sources over tarballs** — shallow clone with tag pinning. Enables
  `yoe dev` workflow (edit, commit, extract patches).
- **apk** package manager (same as Alpine, but with glibc)
- **bubblewrap** for per-recipe build isolation inside the container
- **Layer path** — layers can live in a subdirectory of a repo via the `path`
  field on `layer()`. Layer name is derived from the path's last component.
- **Image deps in DAG** — image recipes' `packages` list is treated as
  dependencies so `yoe build dev-image` automatically builds all required
  packages first
- **Native builds only** — no cross-compilation
- **Label-based references** —
  `load("@recipes-core//classes/autotools.star", "autotools")`, `//` relative to
  layer root when inside a layer
- **Two-phase build** — resolve DAG then execute (inspired by GN)
- **Content-addressed caching** — input hash determines output
- **Hardware-bootable images** — images must boot on real hardware, not just
  QEMU. Never suggest QEMU-only shortcuts like `-kernel` direct boot that bypass
  the bootloader. QEMU is a development convenience; the real target is always
  physical boards.

## Working on This Codebase

- **No shortcuts.** Build systems are fragile. Always implement the correct fix,
  not a workaround that happens to make things pass. If the correct fix is
  significantly harder, explain the trade-off and ask before taking a shortcut.
- **Understand before changing.** Read the relevant code paths end-to-end before
  proposing changes. Build failures often have non-obvious root causes — trace
  the actual problem rather than patching symptoms.
- **Silent failures are bugs.** If something can fail, it should fail loudly
  with a clear error. Never swallow errors or degrade silently in ways that make
  debugging harder later.
