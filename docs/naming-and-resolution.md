# Naming and Resolution

How layers, units, and dependencies are named, referenced, and resolved in
Yoe-NG. This document covers the current model and open design questions.

See [metadata-format.md](metadata-format.md) for the full unit/class/layer
Starlark API. See [build-environment.md](build-environment.md) for how build
isolation and caching work.

## Layers

A **layer** is a Git repository (or subdirectory of one) that provides units,
classes, machine definitions, and images. Layers are declared in `PROJECT.star`:

```python
project(
    name = "my-product",
    layers = [
        layer("https://github.com/YoeDistro/yoe-ng.git",
              ref = "main",
              path = "layers/units-core"),
        layer("https://github.com/vendor/bsp-imx8.git",
              ref = "v2.1.0"),
    ],
)
```

**Layer name** is derived from the `path` field's last component if set,
otherwise the URL's repository name. Examples:

| URL | path | Derived name |
| --- | ---- | ------------ |
| `github.com/YoeDistro/yoe-ng.git` | `layers/units-core` | `units-core` |
| `github.com/vendor/bsp-imx8.git` | (none) | `bsp-imx8` |

Layer names are used in `load()` statements: `load("@units-core//classes/autotools.star", "autotools")`.

### Layer directory structure

```
<layer-root>/
  LAYER.star          # layer metadata and dependencies
  classes/            # build pattern functions (autotools, cmake, etc.)
  units/              # unit definitions (.star files)
  machines/           # machine definitions (.star files)
  images/             # image definitions (.star files)
```

### Evaluation order

1. **Phase 1** — `PROJECT.star` is evaluated. Layers are synced (cloned/fetched).
2. **Phase 1b** — Machine definitions from all layers are evaluated.
3. **Phase 2** — Units and images from all layers are evaluated. `ARCH`,
   `MACHINE`, `MACHINE_CONFIG`, and `PROVIDES` are available as predeclared
   variables.

Within each phase, layers are evaluated in declaration order. Within a layer,
`.star` files are evaluated in filesystem walk order.

## Units

A **unit** is a named build definition declared via `unit()`, `image()`, or a
class function like `autotools()` or `cmake()`. Each unit produces one or more
`.apk` packages.

### Current naming model

Unit names are **flat strings** with no layer namespace:

```python
# In units-core layer:
unit(name = "zstd", ...)

# In another layer:
unit(name = "zstd", ...)  # silently overwrites the first!
```

Currently, if two layers define a unit with the same name, the last one evaluated
wins silently. The DAG, build cache (`build/<arch>/<unit>/`), and APK repo all
key on the unit name, so a collision produces incorrect builds with no error.

## Dependencies

Units declare two kinds of dependencies:

- **`deps`** — build-time. The dependency's output is available in the build
  sysroot during compilation. Resolved by the `yoe` DAG.
- **`runtime_deps`** — install-time. Recorded in the `.apk` package metadata and
  resolved by `apk` during image assembly or on-device install.

Both reference units by name:

```python
autotools(
    name = "curl",
    deps = ["openssl", "zlib", "zstd"],
    runtime_deps = ["openssl", "zlib", "zstd"],
)
```

### Transitive dependencies

Build-time deps are resolved transitively by the DAG. If `curl` depends on
`openssl` and `openssl` depends on `zlib`, curl's build sysroot includes both.

Runtime deps are resolved transitively by `apk` at install time.

## Load references

Starlark `load()` statements use three forms:

| Form | Resolves to | Example |
| ---- | ----------- | ------- |
| `@layer//path` | Named layer root | `load("@units-core//classes/autotools.star", "autotools")` |
| `//path` | Current layer root (context-aware) | `load("//classes/cmake.star", "cmake")` |
| `relative/path` | Relative to current file | `load("../utils.star", "helper")` |

The `//` form is context-aware: if the file is inside a layer, `//` resolves to
that layer's root. Otherwise it resolves to the project root. This means a unit
in `units-core` can `load("//classes/autotools.star", ...)` and it resolves
within `units-core`, not the project root.

## Virtual packages (PROVIDES)

The `PROVIDES` predeclared variable maps virtual names to concrete unit names.
This allows images to reference abstract capabilities rather than specific units:

```python
# Machine definition contributes:
machine(
    name = "raspberrypi4",
    kernel = kernel(unit = "linux-rpi4", provides = "linux"),
)

# Unit can also declare provides:
unit(name = "linux-rpi4", provides = "linux", ...)

# Image uses the virtual name:
image(name = "base-image", artifacts = ["busybox", "linux"], ...)
# "linux" resolves to "linux-rpi4" via PROVIDES
```

`PROVIDES` is populated in two stages:

1. After phase 1 (machines) — `kernel.provides` entries are added
2. After phase 2 (units) — unit `provides` fields are added

If two units provide the same virtual name, the last one evaluated wins (same
silent-overwrite problem as unit names).

## Layer composition

Layers extend upstream units without modifying them by importing the unit as a
callable function:

```python
# @units-core provides openssh as a function
def openssh(extra_deps=[], **overrides):
    autotools(name = "openssh", deps = ["zlib", "openssl"] + extra_deps, **overrides)

# @vendor-bsp extends it
load("@units-core//openssh.star", "openssh")
openssh(extra_deps=["vendor-crypto"])
```

This is explicit and traceable — `grep` for the function call to find all
modifications. See [metadata-format.md](metadata-format.md) for details.

---

## Open Issues

### 1. Unit name collisions across layers

**Problem:** Two layers can define a unit with the same name. The second silently
overwrites the first. No error, no warning.

**Options:**

- **(a) Error on duplicate names.** Simple, catches the problem at eval time.
  Forces layers to coordinate names.
- **(b) Namespace units by layer.** Unit names become `layer/unit` (e.g.,
  `units-core/zstd`). Requires `provides` to map virtual names for deps and
  image artifacts. More explicit but changes every reference.
- **(c) Allow intentional overrides.** A layer can explicitly replace a unit from
  another layer (like Yocto's `.bbappend`). Unintentional duplicates still error.

**Current leaning:** Option (b) — namespace by layer, use `provides` for the
short name. This makes provenance clear in the build dir
(`build/x86_64/units-core/zstd/`) and in logs, and forces explicit resolution
when two layers provide the same capability.

### 2. PROVIDES collision detection

**Problem:** If two units both `provides = "linux"`, the last one wins silently.

**Proposed fix:** Error when two units provide the same virtual name. The machine
definition's `kernel.provides` takes precedence (since it's the machine owner's
choice), but two units claiming the same provides without a machine override
should be an error.

### 3. Dependency references with namespaced units

**Problem:** If unit names become `units-core/zstd`, how do `deps` and
`runtime_deps` reference them?

**Options:**

- **(a) Always use virtual names.** Every unit must `provides` a short name.
  `deps = ["zstd"]` resolves through `PROVIDES`. Forces every unit to declare
  `provides`.
- **(b) Support both forms.** `deps = ["zstd"]` checks `PROVIDES` first, falls
  back to exact name match. `deps = ["units-core/zstd"]` is an explicit
  reference. Provides is optional — only needed when multiple layers offer the
  same capability.
- **(c) Auto-provide the short name.** If only one layer has a unit named
  `units-core/zstd`, automatically register `provides = "zstd"`. Only require
  explicit `provides` when there's ambiguity.

**Current leaning:** Option (c) — auto-provide when unambiguous. This keeps
existing unit definitions simple (no `provides` field needed for most units) and
only requires intervention when there's an actual conflict.

### 4. Build directory layout with namespaced units

**Current:** `build/<arch>/<unit>/` (e.g., `build/x86_64/zstd/`)

**With namespacing:** `build/<arch>/<layer>/<unit>/` (e.g.,
`build/x86_64/units-core/zstd/`)

This is a breaking change for existing build caches. Requires `yoe clean --all`.

### 5. Image artifact references

**Problem:** Image `artifacts` lists use unit names. With namespacing, should
images reference virtual names or full names?

```python
# Virtual names (resolved through PROVIDES):
image(artifacts = ["busybox", "linux", "openssh"])

# Full names:
image(artifacts = ["units-core/busybox", "units-core/linux", "vendor-bsp/openssh"])
```

**Current leaning:** Virtual names in images, full names only when disambiguation
is needed. Images describe _what_ should be installed, not _where_ it comes from.
