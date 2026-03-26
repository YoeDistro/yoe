# Recipe & Configuration Format

Yoe-NG uses [Starlark](https://github.com/google/starlark-go) — a deterministic,
sandboxed dialect of Python — for all build definitions. Recipes, classes,
machine definitions, and project configuration are all `.star` files. See
[Build Languages](build-languages.md) for the rationale behind this choice.

## Recipes vs. Packages

These are distinct concepts in Yoe-NG:

- **Recipes** — `.star` files in the project tree that describe _how to build_
  software. They live in version control and are a development/CI concern.
- **Packages** — `.apk` files that recipes _produce_. They are installable
  artifacts published to a repository and consumed by `apk` during image
  assembly or on-device updates.

The build flow is: **recipe → build → .apk package(s) → repository → image /
device**.

Recipes are inputs to the build system. Packages are outputs. A developer edits
recipes; a device only ever sees packages.

### Sub-packages

A single recipe can produce multiple `.apk` packages. This is the same concept
as Yocto's `PACKAGES` splitting and Debian's binary packages — one source build
produces granular installable units:

| Sub-package | Contents                               | Typical consumer        |
| ----------- | -------------------------------------- | ----------------------- |
| `openssh`   | Binaries, default config               | Production images       |
| `-dev`      | Headers, pkg-config files, static libs | Build-time dependencies |
| `-doc`      | Man pages, info pages                  | Development images      |
| `-dbg`      | Debug symbols (DWARF)                  | Debug/development       |

**In a recipe:**

```python
load("//classes/autotools.star", "autotools")

autotools(
    name = "openssh",
    version = "9.6p1",
    source = "https://cdn.openbsd.org/.../openssh-9.6p1.tar.gz",
    deps = ["zlib", "openssl"],
    # Sub-package splitting — default splits are automatic, but can be
    # customized or disabled per recipe.
    subpackages = {
        "dev": auto(),          # headers + pkg-config (automatic)
        "doc": auto(),          # man pages (automatic)
        "dbg": auto(),          # debug symbols (automatic)
        "server": subpackage(   # custom split
            description = "OpenSSH server",
            files = ["/usr/sbin/sshd", "/etc/ssh/sshd_config"],
            services = ["sshd"],
        ),
        "client": subpackage(
            description = "OpenSSH client utilities",
            files = ["/usr/bin/ssh", "/usr/bin/scp", "/usr/bin/sftp"],
        ),
    },
)
```

**How it works:**

1. The recipe builds once, installing everything into `$DESTDIR`.
2. After the build, the `yoe` engine splits `$DESTDIR` into sub-packages based
   on the `subpackages` declaration.
3. `auto()` splits use file-path conventions (same as Alpine's apk and Yocto's
   `PACKAGES_DYNAMIC`):
   - `-dev`: `/usr/include/**`, `/usr/lib/*.a`, `/usr/lib/pkgconfig/**`
   - `-doc`: `/usr/share/man/**`, `/usr/share/doc/**`, `/usr/share/info/**`
   - `-dbg`: `/usr/lib/debug/**` (debug symbols separated by `strip`)
4. Custom splits use explicit file lists.
5. Each sub-package becomes a separate `.apk` in the repository.

**Default behavior:** If `subpackages` is omitted, automatic `-dev`, `-doc`, and
`-dbg` splits are applied. To produce a single unsplit package, set
`subpackages = {}`.

**In image recipes:**

```python
image(
    name = "production-image",
    packages = [
        "openssh-server",       # just the server, not the full openssh
        "networkmanager",
    ],
)

image(
    name = "dev-image",
    packages = [
        "openssh",              # full package
        "openssh-dev",          # headers for on-device compilation
        "openssh-doc",          # man pages
        "gdb",
    ],
)
```

Sub-packages keep production images small (no headers, no man pages, no debug
symbols) while making development images fully featured. This is the same
tradeoff Yocto and Debian make — granular packaging trades a small amount of
recipe complexity for significant control over image contents.

Alpine's apk already supports sub-packages natively (Alpine's `openssh` APKBUILD
produces `openssh`, `openssh-doc`, `openssh-dev`, etc.), so Yoe-NG follows a
proven pattern.

## Why Starlark

- **One language** — recipes, classes, machines, and project config are all
  `.star` files. No TOML + shell + something-else stack.
- **Python-like syntax** — most developers can read it immediately.
- **Deterministic** — no side effects, no mutable global state. Critical for
  content-addressed caching.
- **Sandboxed** — recipes cannot perform arbitrary I/O or network access.
- **Go-native** — the `go.starlark.net` library embeds directly in the `yoe`
  binary.
- **Composable** — functions, `load()`, and `**kwargs` provide natural
  composition for layers and overrides.
- **Battle-tested** — used by Bazel (Google), Buck2 (Meta), and Pants.

## Recipe Types

### Machine Definition (`machines/<name>.star`)

Describes a target board or platform.

```python
machine(
    name = "beaglebone-black",
    arch = "arm64",
    description = "BeagleBone Black (AM3358)",
    kernel = kernel(
        repo = "https://github.com/beagleboard/linux.git",
        branch = "6.6",
        defconfig = "bb.org_defconfig",
        device_trees = ["am335x-boneblack.dtb"],
    ),
    bootloader = uboot(
        repo = "https://github.com/beagleboard/u-boot.git",
        branch = "v2024.01",
        defconfig = "am335x_evm_defconfig",
    ),
)
```

QEMU machines include emulation configuration:

```python
machine(
    name = "qemu-x86_64",
    arch = "x86_64",
    kernel = kernel(
        recipe = "linux-qemu",
        cmdline = "console=ttyS0 root=/dev/vda2 rw",
    ),
    qemu = qemu_config(
        machine = "q35",
        cpu = "host",
        memory = "1G",
        firmware = "ovmf",
        display = "none",
    ),
)
```

### Image Recipe (`recipes/<name>.star`)

An image is a recipe that assembles a root filesystem from packages and produces
a disk image. Image recipes use the `image()` class function instead of
`package()`. They participate in the same DAG, use the same caching, and are
built with `yoe build`.

```python
load("//classes/image.star", "image")

image(
    name = "base-image",
    version = "1.0.0",
    description = "Minimal bootable system",
    # Packages installed into the rootfs.
    # The base system (glibc, busybox, systemd) is implicit unless excluded.
    packages = [
        "openssh",
        "networkmanager",
        "myapp",
        "monitoring-agent",
    ],
    hostname = "yoe",
    timezone = "UTC",
    locale = "en_US.UTF-8",
    services = ["sshd", "NetworkManager", "myapp"],
    partitions = [
        partition(label="boot", type="vfat", size="64M",
                  contents=["MLO", "u-boot.img", "zImage", "*.dtb"]),
        partition(label="rootfs", type="ext4", size="fill", root=True),
    ],
)
```

### Image Composition and Variants

Image variants use plain Starlark variables and list concatenation — no special
inheritance mechanism:

```python
load("//classes/image.star", "image")

BASE_PACKAGES = [
    "openssh",
    "networkmanager",
    "myapp",
    "monitoring-agent",
]

BASE_SERVICES = ["sshd", "NetworkManager", "myapp"]

BBB_PARTITIONS = [
    partition(label="boot", type="vfat", size="64M",
              contents=["MLO", "u-boot.img", "zImage", "*.dtb"]),
    partition(label="rootfs", type="ext4", size="fill", root=True),
]

image(
    name = "base-image",
    version = "1.0.0",
    packages = BASE_PACKAGES,
    services = BASE_SERVICES,
    partitions = BBB_PARTITIONS,
    hostname = "yoe",
)

image(
    name = "dev-image",
    version = "1.0.0",
    description = "Development image with debug tools",
    packages = BASE_PACKAGES + ["gdb", "strace", "tcpdump", "vim"],
    exclude = ["monitoring-agent"],
    services = BASE_SERVICES,
    partitions = BBB_PARTITIONS,
    hostname = "yoe-dev",
)
```

**Conditional packages per machine:**

```python
packages = ["openssh", "myapp"]
if machine.arch == "arm64":
    packages += ["arm64-firmware"]
```

### Package Recipe (`recipes/<name>.star`)

Describes how to build a system-level package (C/C++ libraries, system daemons,
etc.) and produce an `.apk`. Uses a class function like `autotools()`,
`cmake()`, or the generic `package()`.

```python
load("//classes/autotools.star", "autotools")

autotools(
    name = "openssh",
    version = "9.6p1",
    description = "OpenSSH client and server",
    license = "BSD",
    source = "https://cdn.openbsd.org/pub/OpenBSD/OpenSSH/portable/openssh-9.6p1.tar.gz",
    sha256 = "...",
    configure_args = ["--sysconfdir=/etc/ssh"],
    deps = ["zlib", "openssl"],
    runtime_deps = ["zlib", "openssl"],
    services = ["sshd"],
    conffiles = ["/etc/ssh/sshd_config"],
)
```

Or using the generic `package()` for custom build steps:

```python
package(
    name = "openssh",
    version = "9.6p1",
    source = "https://cdn.openbsd.org/pub/OpenBSD/OpenSSH/portable/openssh-9.6p1.tar.gz",
    sha256 = "...",
    deps = ["zlib", "openssl"],
    runtime_deps = ["zlib", "openssl"],
    build = [
        "./configure --prefix=$PREFIX --sysconfdir=/etc/ssh",
        "make -j$NPROC",
        "make DESTDIR=$DESTDIR install",
    ],
    services = ["sshd"],
    conffiles = ["/etc/ssh/sshd_config"],
)
```

### Patches

Recipes can apply patches to upstream source after fetching and before building.
Patches are listed in order and applied with `git apply` or `patch -p1`:

```python
package(
    name = "busybox",
    version = "1.36.1",
    source = "https://busybox.net/downloads/busybox-1.36.1.tar.bz2",
    patches = [
        "patches/busybox/fix-ash-segfault.patch",
        "patches/busybox/add-custom-applet.patch",
    ],
    build = ["make -j$NPROC", "make DESTDIR=$DESTDIR install"],
)
```

Patch file paths are relative to the project root. Patch contents are included
in the recipe's cache hash — changing a patch triggers a rebuild.

**Layer overrides for patches** work through the standard function composition
pattern:

```python
# upstream: @recipes-core/busybox.star
def busybox(extra_patches=[], **overrides):
    package(
        name = "busybox",
        version = "1.36.1",
        source = "https://busybox.net/downloads/busybox-1.36.1.tar.bz2",
        patches = [
            "patches/busybox/fix-ash-segfault.patch",
        ] + extra_patches,
        build = ["make -j$NPROC", "make DESTDIR=$DESTDIR install"],
        **overrides,
    )

# vendor layer: adds a patch without modifying upstream
load("@recipes-core//busybox.star", "busybox")
busybox(extra_patches=["patches/vendor-busybox-audit.patch"])
```

**Alternatives to patches:**

- **Git-based sources** — fork the repo, apply changes as commits, point the
  recipe at your branch/tag. Cleaner history, easier to rebase on upstream
  updates.
- **Overlay files** — for config file changes on the target, the `overlays/`
  directory is simpler than patching source.

### Application Recipe (`recipes/<name>.star`)

Applications built with language-native build systems use language-specific
class functions that delegate to the language toolchain.

```python
load("//classes/go.star", "go_binary")

go_binary(
    name = "myapp",
    version = "1.2.3",
    description = "Edge data collection service",
    license = "Apache-2.0",
    source = "https://github.com/example/myapp.git",
    tag = "v1.2.3",
    package = "./cmd/myapp",
    services = ["myapp"],
    conffiles = ["/etc/myapp/config.toml"],
    environment = {"DATA_DIR": "/var/lib/myapp"},
)
```

Language-specific classes handle the build details — `go_binary()` sets up
`GOMODCACHE`, runs `go build`, and packages the result. Similar classes exist
for Rust (`rust_binary()`), Zig (`zig_binary()`), Python (`python_package()`),
and Node.js (`node_package()`).

### Project Configuration (`PROJECT.star`)

Top-level configuration that ties everything together.

```python
project(
    name = "yoe",
    version = "0.1.0",
    description = "Yoe-NG embedded Linux distribution",
    defaults = defaults(
        machine = "qemu-arm64",
        image = "base-image",
    ),
    repository = repository(
        path = "/var/cache/yoe-ng/repo",
    ),
    cache = cache(
        path = "/var/cache/yoe-ng/build",
        remote = [
            s3_cache(
                name = "team",
                bucket = "yoe-cache",
                endpoint = "https://minio.internal:9000",
                region = "us-east-1",
            ),
        ],
        retention_days = 90,
        signing = "keys/cache.pub",
    ),
    sources = sources(
        go_proxy = "https://proxy.golang.org",
    ),
    layers = [
        layer("github.com/yoe/recipes-core", ref = "v1.0.0"),
        layer("github.com/vendor/bsp-recipes", ref = "main"),
    ],
)
```

## Classes

Classes are Starlark functions that define build pipelines for different recipe
types. They encapsulate the _how to build_ logic so that recipes only declare
_what to build_.

### Built-in Classes

These ship with `yoe` and cover common build patterns:

| Class              | Description                                   |
| ------------------ | --------------------------------------------- |
| `package()`        | Generic package — custom build steps as shell |
| `autotools()`      | configure / make / make install               |
| `cmake()`          | CMake build                                   |
| `meson()`          | Meson + Ninja build                           |
| `go_binary()`      | Go application                                |
| `rust_binary()`    | Rust application (Cargo)                      |
| `zig_binary()`     | Zig application                               |
| `python_package()` | Python package (pip/uv)                       |
| `node_package()`   | Node.js package (npm/pnpm)                    |
| `image()`          | Root filesystem image assembly                |

### Class Composition

Classes compose through function calls. A recipe can use multiple classes, and
classes can wrap other classes:

```python
load("//classes/autotools.star", "autotools")
load("//classes/systemd.star", "systemd_service")

# Use both autotools and systemd classes
autotools(
    name = "openssh",
    version = "9.6p1",
    configure_args = ["--sysconfdir=/etc/ssh"],
    deps = ["zlib", "openssl"],
)

systemd_service(
    name = "openssh",
    unit = "sshd.service",
    conffiles = ["/etc/ssh/sshd_config"],
)
```

Or create a combined class:

```python
# classes/systemd_autotools.star
load("//classes/autotools.star", "autotools")
load("//classes/systemd.star", "systemd_service")

def systemd_autotools(name, unit, conffiles=[], **kwargs):
    autotools(name=name, **kwargs)
    systemd_service(name=name, unit=unit, conffiles=conffiles)
```

### Custom Classes

Projects can define their own classes in `classes/` for patterns specific to
their codebase:

```python
# classes/my_go_service.star
load("//classes/go.star", "go_binary")
load("//classes/systemd.star", "systemd_service")

def my_go_service(name, version, source, **kwargs):
    """Standard pattern for our Go microservices."""
    go_binary(
        name = name,
        version = version,
        source = source,
        **kwargs,
    )
    systemd_service(
        name = name,
        unit = name + ".service",
        conffiles = ["/etc/" + name + "/config.toml"],
    )
```

### Extensibility: Starlark and Go

Starlark is not a standalone language — it runs embedded inside the `yoe` Go
binary. Every built-in function (`package()`, `machine()`, `image()`, etc.) is a
Go function registered into the Starlark environment. When Starlark code calls
`package(name="openssh", ...)`, it executes Go code that has full access to the
host runtime.

This means the system is extensible in two directions:

**Go to Starlark (primitives):** The `yoe` binary provides built-in functions
that Starlark code can call. These have capabilities Starlark alone cannot —
filesystem I/O, network access, executing system tools (apk, bwrap, git),
managing the build engine state. Adding a new built-in is a Go function with the
right signature:

```go
// In Go: register a new built-in function
func (e *Engine) fnDeploy(thread *starlark.Thread, fn *starlark.Builtin,
    args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
    target := kwString(kwargs, "target")
    // Full access to Go runtime — HTTP, filesystem, exec, etc.
    return starlark.None, nil
}

// Register it in builtins():
"deploy": starlark.NewBuiltin("deploy", e.fnDeploy),
```

Now any `.star` file can call `deploy(target="production")`.

**Starlark to Starlark (composition):** Users define functions in `.star` files
that compose the Go-provided primitives. Classes, macros, and helpers are just
Starlark functions that call built-in functions:

```python
# classes/my_service.star — user-defined class wrapping Go builtins
def my_service(name, version, **kwargs):
    go_binary(name=name, version=version, **kwargs)  # calls Go
    systemd_service(name=name, unit=name + ".service")  # calls Go
```

**The architecture mirrors Bazel:** Go provides the **primitives** (package
creation, image assembly, sandbox execution, cache management), Starlark
provides the **composition layer** (classes, conditionals, layer overrides,
shared variables). Starlark code cannot perform arbitrary I/O — it can only call
the Go functions that `yoe` explicitly exposes, maintaining the sandboxed,
deterministic evaluation model.

## Directory Structure

A typical Yoe-NG project layout:

```
my-project/
├── PROJECT.star
├── machines/
│   ├── beaglebone-black.star
│   ├── raspberrypi4.star
│   └── qemu-arm64.star
├── recipes/
│   ├── base-image.star         # image() class
│   ├── dev-image.star          # image() class, extends base
│   ├── openssh.star            # autotools() class
│   ├── zlib.star
│   ├── openssl.star
│   ├── myapp.star              # go_binary() class
│   └── monitoring-agent.star
├── classes/                    # reusable build rule functions
├── commands/                  # custom yoe subcommands
│   ├── my_go_service.star
│   └── ...
└── overlays/
    └── custom-configs/         # files copied directly into rootfs
        └── etc/
            └── myapp/
                └── config.toml
```

## Build Flow

```
  recipes/*.star               (all recipe types: package and image)
       │
       ▼
  yoe build                    (evaluate Starlark, resolve DAG, build)
       │
       ├─ package() ──▶ compile source ──▶ *.apk packages ──▶ repository/
       │
       └─ image()   ──▶ apk install deps into rootfs
                        ──▶ apply overlays + config
                        ──▶ partition + format
                        ──▶ disk image (.img / .wic)
```

## Layers

Layers are external Git repositories that provide recipes, classes, and machine
definitions. They are the primary mechanism for reusing and sharing build
definitions across projects — BSP vendors ship layers, and product teams compose
them.

### Declaring Layers in PROJECT.star

```python
project(
    name = "my-product",
    version = "1.0.0",
    layers = [
        layer("github.com/yoe/recipes-core", ref = "v1.0.0"),
        layer("github.com/vendor/bsp-imx8", ref = "v2.1.0"),
    ],
)
```

Each `layer()` call declares a Git repository URL and a ref (tag, branch, or
commit SHA). The `yoe` tool fetches and caches these repositories, making them
available as `@layer-name` in `load()` statements.

### Layer Manifests (LAYER.star)

Layers can declare their own dependencies via a `LAYER.star` file in the
repository root. This enables BSP vendors to ship self-contained layers without
requiring users to manually discover transitive dependencies.

```python
# In github.com/vendor/bsp-imx8/LAYER.star
layer_info(
    name = "vendor-bsp-imx8",
    description = "i.MX8 BSP recipes and machine definitions",
    deps = [
        layer("github.com/vendor/hal-common", ref = "v1.3.0"),
        layer("github.com/vendor/firmware-imx", ref = "v5.4"),
    ],
)
```

### Dependency Resolution Rules

Layer dependencies follow the **Go modules model** — the root project has final
authority over versions:

1. **PROJECT.star always wins.** If PROJECT.star and a LAYER.star both reference
   the same repository, the version in PROJECT.star takes precedence. This gives
   the project owner full control over the dependency tree.

2. **Transitive deps are checked, not silently fetched (v1).** In the initial
   implementation, `yoe` reads each layer's `LAYER.star` and **errors** if a
   required dependency is missing from PROJECT.star, rather than silently
   fetching it. The error message tells the user exactly what to add. This is
   explicit and debuggable.

3. **Automatic transitive resolution (v2).** In a future version, transitive
   dependencies declared in `LAYER.star` are fetched automatically when not
   overridden by PROJECT.star. `yoe layer list` shows the full resolved tree so
   nothing is hidden.

4. **Diamond dependencies resolve to the highest version.** If two layers depend
   on different versions of the same repository, `yoe` selects the higher
   version (semver comparison) unless PROJECT.star pins a specific version.

**Example — v1 behavior (missing transitive dep):**

```
$ yoe build --all
Error: layer "vendor-bsp-imx8" requires "github.com/vendor/hal-common" (ref v1.3.0)
       but it is not declared in PROJECT.star.

Add this to your PROJECT.star layers list:
    layer("github.com/vendor/hal-common", ref = "v1.3.0"),
```

**Example — PROJECT.star overriding a transitive version:**

```python
# PROJECT.star
layers = [
    layer("github.com/yoe/recipes-core", ref = "v1.0.0"),
    layer("github.com/vendor/bsp-imx8", ref = "v2.1.0"),
    # Override the version that bsp-imx8 requests (v1.3.0 → v1.4.0)
    layer("github.com/vendor/hal-common", ref = "v1.4.0"),
]
```

### Local Layer Overrides

During development, you often want to work on a layer locally instead of
fetching from Git. The `local` parameter overrides the remote URL:

```python
layers = [
    layer("github.com/yoe/recipes-core", ref = "v1.0.0"),
    # Use a local checkout during development
    layer("github.com/vendor/bsp-imx8", local = "../bsp-imx8"),
]
```

When `local` is set, `yoe` uses the local directory directly (no fetch, no ref
checking). This is equivalent to Go's `replace` directive in `go.mod`.

## Label-Based References

Inspired by Bazel's label system and GN's `//path/to:target`, Yoe-NG uses a
label scheme for referencing recipes and classes across repositories:

```python
# Local references (within the current project)
load("//classes/autotools.star", "autotools")   # from project root
load("//recipes/openssh.star", "openssh_config") # load shared config

# External references (from layers)
load("@recipes-core//openssh.star", "openssh")
load("@vendor-bsp//kernel.star", "vendor_kernel")
```

Layer names (`@recipes-core`, `@vendor-bsp`) map to the layers declared in
`PROJECT.star`. When `yoe` evaluates recipes, it fetches and caches external
layers, then resolves all `load()` references to concrete files.

## Layer Composition

Layers enable the vendor BSP / product overlay pattern without modifying
upstream recipes:

```python
# Layer 1: @recipes-core/openssh.star — base recipe as a function
def openssh(extra_deps=[], extra_configure_args=[], **overrides):
    autotools(
        name = "openssh",
        version = "9.6p1",
        deps = ["zlib", "openssl"] + extra_deps,
        configure_args = ["--sysconfdir=/etc/ssh"] + extra_configure_args,
        **overrides,
    )

# Layer 2: @vendor-bsp/openssh.star — vendor extends it
load("@recipes-core//openssh.star", "openssh")
openssh(extra_deps=["vendor-crypto"])

# Layer 3: product recipe — further customization
load("@vendor-bsp//openssh.star", "openssh")
openssh(extra_configure_args=["--with-pam"])
```

Each layer is explicit about what it modifies and where the base comes from.
This is more traceable than Yocto's bbappend system — you can grep for the
function call to find all modifications.

## Design Notes

- **Starlark over TOML/YAML** — pure data formats accumulate escape hatches
  (conditional deps, shell in strings, inheritance). Starlark makes the implicit
  explicit while remaining readable for simple cases. See
  [Build Languages](build-languages.md) for the full analysis.
- **Prefer git sources over tarballs** — git sources give you upstream history,
  clean `git rebase` for patch updates, natural `yoe dev` workflow (edit, commit,
  extract patches), and no SHA256 to maintain. Use `source = "https://...git"`
  with a `tag` to pin the version.
- **One file per recipe** — each recipe is its own `.star` file. This keeps
  diffs clean and makes it easy to add/remove components.
- **Recipes and packages are separate concerns** — recipes are
  version-controlled build instructions; packages are binary artifacts. This
  separation enables building once and deploying many times, sharing packages
  across teams, and on-device incremental updates via `apk`.
- **Classes as functions** — build patterns (autotools, cmake, go) are Starlark
  functions, not a type system. Multiple classes compose through function calls.
  This is simpler and more flexible than Yocto's class inheritance.
- **Unified recipe directory** — system packages, application packages, and
  images all live in `recipes/`. The class function determines the output:
  `package()` / `autotools()` / etc. produce `.apk` files, `image()` produces
  disk images. One concept (recipe), one directory, one DAG.
- **apk for image assembly** — image recipes declare their packages as
  dependencies. `yoe build <image>` creates a clean rootfs and runs `apk add` to
  populate it from the repository, exactly like Alpine's image builder. This
  leverages apk's dependency resolution rather than reimplementing it.
