# Design: `binary` class for prebuilt-binary units

## Purpose

Many GitHub (and other-host) projects ship official prebuilt arm64 / x86_64
binaries on every release. For **leaf tools** that aren't linked into anything
else `[yoe]` builds — `kubectl`, `helm`, `gh`, `task`, `fly`, `helix`, etc. —
rebuilding from source is wasted work. `[yoe]` should be able to consume the
upstream binary directly.

The `binary` class wraps that pattern: declare base URL, per-arch asset
(literal or templated), and per-arch SHA256 hashes. The class fetches,
integrity-checks, extracts (or copies, for bare binaries), places the primary
binary into the apk, optionally installs additional files / directories from
the archive, and creates any requested symlinks.

This is a **leaf-tool class**. Anything that other units link against, or that
other units need headers from, should still build from source.

## Non-goals

- **Not for libraries.** Output is binaries on `$PATH`, not `.so`/`.h` for
  consumers.
- **Not for closed-source vendor SDKs.** v1 doesn't model EULAs, restricted
  redistribution, or licence-acceptance gates.
- **Not a "build from source if no binary available" fallback.** If the
  project doesn't ship arm64, the unit errors. That's correct.
- **No SHA256SUMS auto-fetch, no Sigstore / cosign verification.** Literal
  per-arch SHA256 is the v1 integrity check.
- **Not a generic "install arbitrary tarballs" class.** The shape is
  opinionated toward "one tool, optionally with runtime support files and
  symlinks."

## API surface

```python
load("//classes/binary.star", "binary")

# 1. Bare single binary (no archive)
binary(
    name = "kubectl",
    version = "1.29.0",
    base_url = "https://dl.k8s.io/release/v1.29.0/bin/linux",
    asset = "{arch}/kubectl",
    sha256 = {"x86_64": "...", "arm64": "..."},
    license = "Apache-2.0",
)

# 2. Single binary inside an archive (template)
binary(
    name = "helm",
    version = "3.14.0",
    base_url = "https://get.helm.sh",
    asset = "helm-v3.14.0-linux-{arch}.tar.gz",
    sha256 = {"x86_64": "...", "arm64": "..."},
    binary_path = "helm",
    license = "Apache-2.0",
)

# 3. Asymmetric per-arch asset names (e.g. Rust target triples)
binary(
    name = "fly",
    version = "0.2.0",
    base_url = "https://github.com/superfly/flyctl/releases/download/v0.2.0",
    assets = {
        "x86_64": "fly-x86_64-unknown-linux-musl.tar.gz",
        "arm64":  "fly-aarch64-unknown-linux-musl.tar.gz",
    },
    sha256 = {"x86_64": "...", "arm64": "..."},
    binary_path = "fly",
    license = "Apache-2.0",
)

# 4. Bundle: directory tree + shortcut symlinks
binary(
    name = "helix",
    version = "24.07",
    base_url = "https://github.com/helix-editor/helix/releases/download/24.07",
    assets = {
        "x86_64": "helix-24.07-x86_64-linux.tar.xz",
        "arm64":  "helix-24.07-aarch64-linux.tar.xz",
    },
    sha256 = {"x86_64": "...", "arm64": "..."},
    binary = "",                                 # disable default $PREFIX/bin install
    extras = [
        ("hx",      "$PREFIX/lib/helix/hx", 0o755),
        ("runtime", "$PREFIX/lib/helix/runtime"),
    ],
    symlinks = {
        "$PREFIX/bin/hx": "../lib/helix/hx",
        "$PREFIX/bin/h":  "../lib/helix/hx",
    },
    license = "MPL-2.0",
)
```

### Field reference

| field | type | required | meaning |
| --- | --- | --- | --- |
| `name`, `version` | str | yes | standard |
| `base_url` | str | yes | URL prefix |
| `asset` | str | one of `asset` / `assets` | single template, with `{arch}` substitution |
| `assets` | dict | one of `asset` / `assets` | per-arch literal asset paths (no templating) |
| `arch_map` | dict | no | yoe arch → token substituted into `{arch}` in `asset`; default `{x86_64: amd64, arm64: arm64}`; ignored when `assets` (literal) is used |
| `sha256` | dict | yes | per-arch literal SHA256; never templated |
| `binary` | str | no | install filename in `$PREFIX/bin`; default `name`; `""` disables the default binary install |
| `binary_path` | str | no | path of the primary binary inside `$SRCDIR` (i.e. inside the extracted archive *after* the existing top-level-directory strip); `{arch}` allowed; default `name` |
| `extras` | list | no | tuples `(src_in_srcdir, dst_in_destdir)` or `(src, dst, mode)`; supports files **and** directories |
| `symlinks` | dict | no | `dst_path → target` (relative or absolute); created after install |
| `license`, `description`, `services`, `conffiles`, `runtime_deps`, `deps`, `tasks`, `scope` | | no | passed through to `unit()` |

`{arch}` substitution applies to `asset`, `binary_path`, and `extras` source
paths. It does **not** apply to `sha256`, `symlinks`, or any `dst` path —
hashes and install destinations are always literal.

### Validation (Starlark eval time)

The class errors immediately, before `unit()` is called, if:

- `ARCH` (predeclared) is not a key in `assets`, `sha256`, or — when `asset`
  is templated — in `arch_map`.
- Neither `asset` nor `assets` is set; or both are set.
- `binary == ""` and `extras` is empty (nothing would be installed).

Error messages name the unit, the arch, and the missing field. No silent
defaults.

## Source preparation: prerequisite Go changes

The existing source pipeline (`internal/source/workspace.go::Prepare` →
`extractTarball`) handles tarballs (`.tar.gz`/`.tar.xz`/`.tar.bz2`/`.tgz`)
with automatic top-level-directory stripping and SHA256 verification. The
`binary` class relies on that machinery, but two gaps need to be filled
before it can work:

1. **`.zip` extraction.** Add zip support to the extraction path so
   `.zip`-packaged releases (common on Windows-leaning projects but also
   present on a few Linux releases) extract into `$SRCDIR` like tarballs do.
   Apply the same auto-strip-top-level-dir behaviour.
2. **Bare-binary "extraction".** Add a path that handles non-archive
   downloads: when the cached file is not a recognised archive (`.tar.*`,
   `.tgz`, `.tbz2`, `.zip`), copy it into `$SRCDIR` as `$SRCDIR/<asset
   basename>` (e.g. `$SRCDIR/kubectl` for asset `amd64/kubectl`), preserving
   the upstream filename, and make it executable. The class then references
   it via `binary_path` like any other archived layout — for the typical
   `name == asset_basename` case the default `binary_path = name` already
   resolves correctly.

   Detection precedence: filename extension first; fall back to magic-byte
   sniffing for files with no/unknown extension. ELF (`7f 45 4c 46`) and any
   non-archive content are treated as bare. Unknown content with an archive
   extension (gzip-with-`.tar.gz`-mismatch, etc.) still goes through the
   archive path and surfaces the existing error.

These changes live in `internal/source/workspace.go` (extraction) and
possibly `internal/source/fetch.go` (extension probe). They don't change
behaviour for any existing unit — every current source is either a git repo
or a tarball with a recognised extension.

## Class internals

1. **Resolve URL and SHA at Starlark eval time.** Read predeclared `ARCH`.
   Validate as above. Compose:
   - `asset_path = assets[ARCH]` if `assets` is set, else
     `asset.replace("{arch}", arch_map[ARCH])`.
   - `source = base_url + "/" + asset_path`.
   - `sha = sha256[ARCH]`.
2. **Call `unit(source=source, sha256=sha, container="toolchain-musl",
   sandbox=False, ...)`.** Cache keying is per-URL, so each arch gets its
   own cache slot for free — no extra changes to the source fetcher.
3. **Generate one `task("install", steps=[...])`.** Because Go-side source
   prep handles all the extraction/copy work, the install task is small:
   - Resolve `{arch}` substitutions in `binary_path` and `extras` source
     paths at Starlark eval time, before generating shell.
   - **Default binary install** (when `binary != ""`):
     `install -m0755 $SRCDIR/$binary_path $DESTDIR$PREFIX/bin/$binary`
     plus `mkdir -p` of the parent.
   - **Extras** (each tuple): `cp -aT $SRCDIR/<src> $DESTDIR<dst>` with
     `mkdir -p` parent; `chmod` if mode supplied. `-aT` preserves directory
     structure and treats `<dst>` as a target name (not a parent).
   - **Symlinks**: `mkdir -p $(dirname dst)` then `ln -sfn <target> <dst>`.

The unit author never sees fetch, extract, hash-verify, or strip logic —
all of that is the platform's job.

## Container

`toolchain-musl` — already has `install`, `cp`, `mkdir`, `ln`. The fetched
binary is never executed at build time, so cross-arch installs (e.g.,
building an arm64 image on an x86_64 host) don't incur QEMU emulation cost
on the binary itself. `sandbox=False` because there's no compile step that
needs bwrap isolation.

## Interaction with existing systems

- **Source fetch** — no changes to fetch.go's HTTP path itself. Existing
  cache-by-URL-hash + SHA verification applies as-is.
- **Source extract** — `internal/source/workspace.go` gains zip + bare-file
  handling (see prerequisite section).
- **Cache** — content-addressed by URL hash + verified SHA, same as other
  HTTP sources. Per-arch URLs naturally produce per-arch cache entries.
- **APK ownership** — output goes through the existing `archive/tar`
  normalisation in `internal/artifact/apk.go` that forces `root:root`. The
  class doesn't think about ownership.
- **DAG / deps** — class adds `toolchain-musl` to `deps` so the container
  unit is in the graph. No other implicit deps.

## Failure modes (intentional, loud)

- **Unsupported arch on this unit**: `ARCH not in sha256` → Starlark error
  with the unit name and the missing arch. Caught at eval, before any fetch.
- **SHA mismatch**: existing fetcher behaviour — fetch fails, file removed
  from cache.
- **Both `asset` and `assets` set, or neither set**: Starlark error.
- **`binary=""` with empty `extras`**: Starlark error ("no install steps
  declared").
- **Missing file in `$SRCDIR` referenced by `binary_path` / `extras`**:
  install task exits non-zero with the path it was looking for.

No silent fall-through anywhere.

## Out of scope for v1 (deferred)

- **Sigstore / cosign verification.** Add when a unit needs it; SHA256 is
  enough for now.
- **SHA256SUMS-file convenience.** A separate helper (or the `new-unit`
  skill) can scrape SHA256SUMS and emit literal hashes into the unit; the
  class itself stays declarative.
- **Auto-discovery of binary path.** v1 requires explicit `binary_path`
  (or its default of `name`). A future "auto" mode could find the single
  executable file in `$SRCDIR`, but that's a hidden default the project
  policy explicitly avoids.
- **Multiple primary binaries with separate `bin/` install of each.** Use
  `binary = ""` + `extras` + `symlinks` for the helix-style bundle case.
  Most projects ship one binary.
- **Custom strip-components.** The existing top-level-strip behaviour is
  what every release tarball needs in practice. If a real unit shows up
  that wants strip=0 or strip>1, add the field then.

## Testing

- **Unit eval tests** — Starlark tests that exercise each form (bare,
  template, asymmetric, bundle) and confirm the class produces a `unit()`
  call with the right `source`/`sha256`/tasks for ARCH=x86_64 and
  ARCH=arm64. Verify the validation errors fire for: missing arch in
  `sha256`, both `asset` and `assets` set, `binary=""` with empty `extras`.
- **Source-prep tests (Go)** — table-driven tests for the new zip path and
  the new bare-binary path: zip with top-level dir, zip flat at root, bare
  ELF, bare statically-linked binary. Existing tarball tests stay green.
- **End-to-end build test** — add at least one real unit using `binary` —
  candidates: `kubectl` (bare binary, single arch token), `helm` (templated
  archive), and ideally one bundle-style example. Run `yoe build` for both
  arches in CI; the produced apk should contain the binary at
  `/usr/bin/<name>` and any declared symlinks pointing at the right
  targets.

## Open questions for follow-up

- **Default `arch_map`.** `{x86_64: amd64, arm64: arm64}` covers the Go
  ecosystem. Rust projects often need different tokens. The class default
  is fine; per-unit `arch_map` overrides cover the rest. Worth revisiting
  if a pattern emerges (e.g., a `RUST_ARCH_MAP` constant exposed by the
  class).
- **Symlink target style.** Examples use relative paths
  (`../lib/helix/hx`). Both relative and absolute work; the class doesn't
  enforce one. Pattern may emerge from real usage.
- **Where bundle installs go.** Examples use `$PREFIX/lib/<name>/`. Some
  projects might prefer `$PREFIX/share/<name>/` or `/opt/<name>/`. The
  class doesn't pick — `extras` destinations are explicit.
