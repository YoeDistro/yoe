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

| URL                               | path                | Derived name |
| --------------------------------- | ------------------- | ------------ |
| `github.com/YoeDistro/yoe-ng.git` | `layers/units-core` | `units-core` |
| `github.com/vendor/bsp-imx8.git`  | (none)              | `bsp-imx8`   |

Layer names are used in `load()` statements:
`load("@units-core//classes/autotools.star", "autotools")`.

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

1. **Phase 1** — `PROJECT.star` is evaluated. Layers are synced
   (cloned/fetched).
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
unit(name = "zstd", ...)  # ERROR: duplicate unit name
```

If two layers define a unit with the same name, the build errors at evaluation
time. To extend an upstream unit, use the
[layer composition](#layer-composition) pattern.

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

| Form            | Resolves to                        | Example                                                    |
| --------------- | ---------------------------------- | ---------------------------------------------------------- |
| `@layer//path`  | Named layer root                   | `load("@units-core//classes/autotools.star", "autotools")` |
| `//path`        | Current layer root (context-aware) | `load("//classes/cmake.star", "cmake")`                    |
| `relative/path` | Relative to current file           | `load("../utils.star", "helper")`                          |

The `//` form is context-aware: if the file is inside a layer, `//` resolves to
that layer's root. Otherwise it resolves to the project root. This means a unit
in `units-core` can `load("//classes/autotools.star", ...)` and it resolves
within `units-core`, not the project root.

## Virtual packages (PROVIDES)

The `PROVIDES` predeclared variable maps virtual names to concrete unit names.
This allows images to reference abstract capabilities rather than specific
units:

```python
# Machine definition contributes:
machine(
    name = "raspberrypi4",
    kernel = kernel(unit = "linux-rpi4", provides = "linux"),
)

# Unit can also declare provides:
unit(name = "linux-rpi4", provides = "linux", ...)

# Image uses the virtual name:
image(name = "base-image", artifacts = ["busybox", "linux", "init"], ...)
# "linux" resolves to "linux-rpi4" via PROVIDES
# "init" resolves to whichever init system the project includes
```

This pattern extends to any swappable core component. For example, the init
system can be abstracted behind a virtual name, with thin configuration layers
providing the concrete implementation:

```python
# layers/config-systemd/units/init.star
unit(name = "systemd", ..., provides = "init")

# layers/config-busybox-init/units/init.star
unit(name = "busybox-init", ..., provides = "init")
```

The project selects which init system to use by including the appropriate layer:

```python
# projects/product-a.star
project(name = "product-a", layers = [
    layer("...", path = "layers/units-core"),
    layer("...", path = "layers/config-systemd"),
])

# projects/product-b.star
project(name = "product-b", layers = [
    layer("...", path = "layers/units-core"),
    layer("...", path = "layers/config-busybox-init"),
])
```

Images reference `init` in their artifacts — they don't need to know whether the
product uses systemd or busybox init.

`PROVIDES` is populated in two stages:

1. After phase 1 (machines) — `kernel.provides` entries are added
2. After phase 2 (units) — unit `provides` fields are added

If two active units provide the same virtual name, the build errors. See
[Collision Detection](#collision-detection) for scoping rules.

## Layer composition

Layers extend upstream units without modifying them by importing the unit as a
callable function:

```python
# @units-core provides openssh as a function with a default name
def openssh(name="openssh", extra_deps=[], **overrides):
    autotools(name = name, deps = ["zlib", "openssl"] + extra_deps, **overrides)

openssh()  # registers "openssh" — units-core works standalone

# @vendor-bsp extends it with a different name
load("@units-core//units/openssh.star", "openssh")
openssh(name = "openssh-vendor", extra_deps = ["vendor-crypto"])
```

The downstream unit has a distinct name (`openssh-vendor`), so there is no
collision with the upstream `openssh`. Images that need the vendor variant
reference `openssh-vendor` in their artifacts list. This is explicit and
traceable — `grep` for the function call to find all extensions. See
[metadata-format.md](metadata-format.md) for details.

---

## Collision Detection

### Unit name duplicates

Unit names are flat strings. If two layers define a unit with the same name, the
build errors at evaluation time. Layers must coordinate names or use the
[layer composition](#layer-composition) pattern to explicitly extend an upstream
unit.

### PROVIDES duplicates

If two **active** units provide the same virtual name, the build errors. The
active set is scoped to the selected machine — units from unselected machines do
not participate. This allows multiple machines to each provide `linux` via
different kernel units without conflict:

```python
# machine/raspberrypi4.star — only active when this machine is selected
machine(name = "raspberrypi4",
    kernel = kernel(unit = "linux-rpi4", provides = "linux"))

# machine/beaglebone.star — only active when this machine is selected
machine(name = "beaglebone",
    kernel = kernel(unit = "linux-bb", provides = "linux"))

# base-image.star — "linux" resolves to whichever kernel the selected machine provides
image(name = "base-image", artifacts = ["busybox", "linux", "openssh"])
```

Images reference provides names directly — no prefix or namespace. The image
declares _what_ should be installed; resolution handles _where_ it comes from.

---

## Open Issues

### Unit replacement via provides

A downstream layer may want to replace an upstream unit transparently — e.g.,
`openssh-vendor` replaces `openssh` in all images without changing image
definitions. The natural mechanism is `provides = "openssh"` on the downstream
unit:

```python
# @vendor-bsp extends openssh and provides the same virtual name
load("@units-core//units/openssh.star", "openssh")
openssh(name = "openssh-vendor", extra_deps = ["vendor-crypto"], provides = "openssh")
```

The problem: the upstream layer already registered a real unit named `openssh`
(via the top-level `openssh()` call). Now both `openssh` (the unit) and
`openssh-vendor` (via provides) map to the name `openssh`. The DAG needs a rule
for which one wins.

This becomes more important with multi-level override chains. A common embedded
pattern is base → SOC → SOM, where each layer extends the previous:

```python
# @units-core//units/base-files.star
def base_files(name="base-files", extra_deps=[], **overrides):
    unit(name = name, deps = ["busybox"] + extra_deps, **overrides)

base_files()

# @soc-layer//units/base-files-soc.star
load("@units-core//units/base-files.star", "base_files")

def base_files_soc(name="base-files-soc", extra_deps=[], **overrides):
    base_files(name = name, extra_deps = ["soc-firmware"] + extra_deps, **overrides)

base_files_soc()

# @som-layer//units/base-files-som.star
load("@soc-layer//units/base-files-soc.star", "base_files_soc")

base_files_soc(name = "base-files-som", extra_deps = ["som-wifi-config"],
               provides = "base-files")
```

The build chain works — each layer extends the previous and produces a distinct
unit name. But images want to reference `base-files`, not `base-files-som`. The
final unit declares `provides = "base-files"`, yet `units-core` already
registered a real unit with that name. All three units (`base-files`,
`base-files-soc`, `base-files-som`) are registered, but only `base-files-som`
should be reachable.

**Preferred approach: use the specific name.** When possible, images and
dependencies should reference the overridden unit by its actual name (e.g.,
`base-files-som` rather than `base-files`). This is clear and explicit — you can
see exactly which unit is being used.

**When a core package must be overridden transparently**, `provides` combined
with layer priority resolves the ambiguity. Layers in the project's layer list
are ordered from lowest to highest priority (last layer wins). When a unit in a
later layer declares `provides = "base-files"`, it takes precedence over the
real unit named `base-files` from an earlier layer:

```python
project(name = "product", layers = [
    layer("...", path = "layers/units-core"),    # lowest priority
    layer("...", path = "layers/soc-layer"),      # overrides units-core
    layer("...", path = "layers/som-layer"),      # highest priority
])
```

With this ordering, `som-layer`'s `base-files-som` (which declares
`provides = "base-files"`) wins over `units-core`'s real `base-files` unit. The
earlier unit becomes unreachable — it is still registered but never pulled into
the DAG because provides from a higher-priority layer takes precedence.

### Projects as layer scoping

A project defines which layers are active for a build. Only units from included
layers participate in the DAG. This is the primary mechanism for controlling
which units can override or conflict with each other — if a layer isn't in the
project's layer list, its units don't exist for that build.

This reduces the collision problem: instead of needing `replaces` or shadow
semantics, a project simply includes only the layers it needs. A vendor layer
that provides its own `openssh-vendor` with `provides = "openssh"` works cleanly
when the project doesn't include a second layer that also provides `openssh`.

A single repository may define multiple projects (similar to KAS YAML files in
yoe-distro), each selecting a different subset of layers for different products
or build configurations:

```python
# projects/dev.star
project(
    name = "dev",
    layers = [
        layer("...", path = "layers/units-core"),
        layer("...", path = "layers/dev-tools"),
    ],
)

# projects/customer-a.star
project(
    name = "customer-a",
    layers = [
        layer("...", path = "layers/units-core"),
        layer("...", path = "layers/vendor-bsp"),
        layer("...", path = "layers/customer-a"),
    ],
)
```

The CLI selects a project: `yoe build --project projects/customer-a.star`.

A default project (`PROJECT.star` at the repo root) can delegate to another
project using standard Starlark `load()`. Two cases:

**Use a project as-is** — load it for the side effect (its `project()` call
registers the project):

```python
# PROJECT.star
load("projects/customer-a.star")
```

**Extend a project with additional layers** — load the exported layer list and
build on it:

```python
# projects/customer-a.star
LAYERS = [
    layer("...", path = "layers/units-core"),
    layer("...", path = "layers/vendor-bsp"),
    layer("...", path = "layers/customer-a"),
]

project(name = "customer-a", layers = LAYERS)

# PROJECT.star
load("projects/customer-a.star", "LAYERS")

project(
    name = "default",
    layers = LAYERS + [
        layer("...", path = "layers/dev-tools"),
    ],
)
```

This lets a developer run `yoe build` without specifying `--project` while
keeping per-product project definitions separate. No new concepts needed —
Starlark's `load()` handles composition naturally.

### APK repo scoping per project

The APK repo must be scoped per project. If two projects share a single repo
(e.g., one uses systemd, the other busybox-init), switching projects leaves
stale packages in the APKINDEX. Since `apk` resolves runtime dependencies from
the index, it could transitively pull in packages from the wrong project.

Build output is scoped as:

```
repo/<project>/APKINDEX.tar.gz
```

Each project gets a clean repo containing only packages from its resolved layer
and unit set. Individual unit builds are still cached by content hash — if two
projects build the same unit with the same inputs, the build runs once and the
resulting apk is placed into both project repos.

The build cache handles provides swapouts automatically: each unit's cache key
includes the hashes of its resolved dependencies (recursively). When `init`
resolves to `systemd` in one project but `busybox-init` in another, any unit
that depends on `init` gets a different cache key because the resolved
dependency's hash differs. No special virtual-name logic is needed in the hasher
— it just hashes the resolved unit, not the virtual name string.
