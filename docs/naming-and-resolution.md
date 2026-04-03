# Naming and Resolution

How modules, units, and dependencies are named, referenced, and resolved in
Yoe-NG.

See [metadata-format.md](metadata-format.md) for the full unit/class/module
Starlark API. See [build-environment.md](build-environment.md) for how build
isolation and caching work.

## Modules

A **module** is a Git repository (or subdirectory of one) that provides units,
classes, machine definitions, and images. Modules are declared in
`PROJECT.star`:

```python
project(
    name = "my-product",
    modules = [
        module("https://github.com/YoeDistro/yoe-ng.git",
              ref = "main",
              path = "modules/units-core"),
        module("https://github.com/vendor/bsp-imx8.git",
              ref = "v2.1.0"),
    ],
)
```

**Module name** is derived from the `path` field's last component if set,
otherwise the URL's repository name. Examples:

| URL                               | path                 | Derived name |
| --------------------------------- | -------------------- | ------------ |
| `github.com/YoeDistro/yoe-ng.git` | `modules/units-core` | `units-core` |
| `github.com/vendor/bsp-imx8.git`  | (none)               | `bsp-imx8`   |

Module names are used in `load()` statements:
`load("@units-core//classes/autotools.star", "autotools")`.

### Module directory structure

```
<module-root>/
  MODULE.star         # module metadata and dependencies
  classes/            # build pattern functions (autotools, cmake, etc.)
  units/              # unit definitions (.star files)
  machines/           # machine definitions (.star files)
  images/             # image definitions (.star files)
```

### Evaluation order

1. **Phase 1** — `PROJECT.star` is evaluated. Modules are synced
   (cloned/fetched).
2. **Phase 1b** — Machine definitions from all modules are evaluated.
3. **Phase 2** — Units and images from all modules are evaluated. `ARCH`,
   `MACHINE`, `MACHINE_CONFIG`, and `PROVIDES` are available as predeclared
   variables.

Within each phase, modules are evaluated in declaration order. Within a module,
`.star` files are evaluated in filesystem walk order.

## Units

A **unit** is a named build definition declared via `unit()`, `image()`, or a
class function like `autotools()` or `cmake()`. Each unit produces one or more
`.apk` packages.

### Current naming model

Unit names are **flat strings** with no module namespace:

```python
# In units-core module:
unit(name = "zstd", ...)

# In another module:
unit(name = "zstd", ...)  # ERROR: duplicate unit name
```

If two modules define a unit with the same name, the build errors at evaluation
time. To extend an upstream unit, use the
[module composition](#module-composition) pattern.

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

| Form            | Resolves to                         | Example                                                    |
| --------------- | ----------------------------------- | ---------------------------------------------------------- |
| `@module//path` | Named module root                   | `load("@units-core//classes/autotools.star", "autotools")` |
| `//path`        | Current module root (context-aware) | `load("//classes/cmake.star", "cmake")`                    |
| `relative/path` | Relative to current file            | `load("../utils.star", "helper")`                          |

The `//` form is context-aware: if the file is inside a module, `//` resolves to
that module's root. Otherwise it resolves to the project root. This means a unit
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
system can be abstracted behind a virtual name, with thin configuration modules
providing the concrete implementation:

```python
# modules/config-systemd/units/init.star
unit(name = "systemd", ..., provides = "init")

# modules/config-busybox-init/units/init.star
unit(name = "busybox-init", ..., provides = "init")
```

The project selects which init system to use by including the appropriate
module:

```python
# projects/product-a.star
project(name = "product-a", modules = [
    module("...", path = "modules/units-core"),
    module("...", path = "modules/config-systemd"),
])

# projects/product-b.star
project(name = "product-b", modules = [
    module("...", path = "modules/units-core"),
    module("...", path = "modules/config-busybox-init"),
])
```

Images reference `init` in their artifacts — they don't need to know whether the
product uses systemd or busybox init.

`PROVIDES` is populated in two stages:

1. After phase 1 (machines) — `kernel.provides` entries are added
2. After phase 2 (units) — unit `provides` fields are added

See [Collision Detection](#collision-detection) for scoping and priority rules.

### Unit replacement via provides

A downstream module may transparently replace an upstream unit by declaring
`provides` equal to the upstream unit's name. Module priority follows
declaration order in `project()` — later modules have higher priority (last
wins):

```python
project(name = "product", modules = [
    module("...", path = "modules/units-core"),    # lowest priority
    module("...", path = "modules/soc-module"),     # overrides units-core
    module("...", path = "modules/som-module"),     # highest priority
])
```

When a unit in a higher-priority module declares `provides = "base-files"`, it
takes precedence over the real unit named `base-files` from a lower-priority
module. A notice is emitted to stderr. The shadowed unit remains registered but
is unreachable via the virtual name — it will not be pulled into the DAG.

This pattern handles multi-level override chains. A common embedded pattern is
base → SOC → SOM, where each module extends the previous:

```python
# @units-core//units/base-files.star
def base_files(name="base-files", extra_deps=[], **overrides):
    unit(name = name, deps = ["busybox"] + extra_deps, **overrides)

base_files()

# @soc-module//units/base-files-soc.star
load("@units-core//units/base-files.star", "base_files")

def base_files_soc(name="base-files-soc", extra_deps=[], **overrides):
    base_files(name = name, extra_deps = ["soc-firmware"] + extra_deps, **overrides)

base_files_soc()

# @som-module//units/base-files-som.star
load("@soc-module//units/base-files-soc.star", "base_files_soc")

base_files_soc(name = "base-files-som", extra_deps = ["som-wifi-config"],
               provides = "base-files")
```

All three units (`base-files`, `base-files-soc`, `base-files-som`) are
registered, but only `base-files-som` is reachable via the `base-files` virtual
name. Images reference `base-files` and automatically get the most-derived
variant.

**When possible, prefer explicit names.** Images and dependencies that reference
the specific name (e.g., `base-files-som`) are clearer and more traceable. Use
transparent replacement only when a core package must be overridden without
changing image definitions.

## Module composition

Modules extend upstream units without modifying them by importing the unit as a
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

Unit names are flat strings. If two modules define a unit with the same name,
the build errors at evaluation time with a message showing which module first
defined the unit. Modules must coordinate names or use the
[module composition](#module-composition) pattern to explicitly extend an
upstream unit.

### PROVIDES duplicates

If two units from the **same module** provide the same virtual name, the build
errors. If two units from **different modules** provide the same virtual name,
the higher-priority module (later in the module list) wins and a notice is
emitted to stderr. The active set is scoped to the selected machine — units from
unselected machines do not participate. This allows multiple machines to each
provide `linux` via different kernel units without conflict:

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

## Projects as module scoping

A project defines which modules are active for a build. Only units from included
modules participate in the DAG. This is the primary mechanism for controlling
which units can override or conflict with each other — if a module isn't in the
project's module list, its units don't exist for that build.

This reduces the collision problem: instead of needing `replaces` or shadow
semantics, a project simply includes only the modules it needs. A vendor module
that provides its own `openssh-vendor` with `provides = "openssh"` works cleanly
when the project doesn't include a second module that also provides `openssh`.

A single repository may define multiple projects (similar to KAS YAML files in
yoe-distro), each selecting a different subset of modules for different products
or build configurations:

```python
# projects/dev.star
project(
    name = "dev",
    modules = [
        module("...", path = "modules/units-core"),
        module("...", path = "modules/dev-tools"),
    ],
)

# projects/customer-a.star
project(
    name = "customer-a",
    modules = [
        module("...", path = "modules/units-core"),
        module("...", path = "modules/vendor-bsp"),
        module("...", path = "modules/customer-a"),
    ],
)
```

The `--project` flag selects a project file:
`yoe --project projects/customer-a.star build`. It is available on all
subcommands. When omitted, `yoe` uses `PROJECT.star` at the repo root.

A default project (`PROJECT.star` at the repo root) can delegate to another
project using standard Starlark `load()`. Two cases:

**Use a project as-is** — load it for the side effect (its `project()` call
registers the project):

```python
# PROJECT.star
load("projects/customer-a.star")
```

**Extend a project with additional modules** — load the exported module list and
build on it:

```python
# projects/customer-a.star
MODULES = [
    module("...", path = "modules/units-core"),
    module("...", path = "modules/vendor-bsp"),
    module("...", path = "modules/customer-a"),
]

project(name = "customer-a", modules = MODULES)

# PROJECT.star
load("projects/customer-a.star", "MODULES")

project(
    name = "default",
    modules = MODULES + [
        module("...", path = "modules/dev-tools"),
    ],
)
```

This lets a developer run `yoe build` without specifying `--project` while
keeping per-product project definitions separate. No new concepts needed —
Starlark's `load()` handles composition naturally.

## Per-project APK repo

The APK repo is scoped per project. If two projects share a single repo (e.g.,
one uses systemd, the other busybox-init), switching projects would leave stale
packages in the APKINDEX. Since `apk` resolves runtime dependencies from the
index, it could transitively pull in packages from the wrong project.

Build output is scoped as:

```
repo/<project>/APKINDEX.tar.gz
```

Each project gets a clean repo containing only packages from its resolved module
and unit set. Individual unit builds are still cached by content hash — if two
projects build the same unit with the same inputs, the build runs once and the
resulting apk is placed into both project repos.

The build cache handles provides swapouts automatically: each unit's cache key
includes the hashes of its resolved dependencies (recursively). When `init`
resolves to `systemd` in one project but `busybox-init` in another, any unit
that depends on `init` gets a different cache key because the resolved
dependency's hash differs. No special virtual-name logic is needed in the hasher
— it just hashes the resolved unit, not the virtual name string.
