# SDK Management

How Yoe-NG provides development environments for application and system
developers targeting embedded hardware.

## The Problem

Embedded Linux development traditionally requires an SDK — a cross-compiler,
target sysroot, and headers/libraries — distributed to developers who write
applications for the target device. Yocto's `populate_sdk` generates a
self-contained tarball with everything needed to cross-compile for the target.

Yoe-NG does not cross-compile. All builds are native — the host architecture
_is_ the target architecture. This changes the SDK story fundamentally: instead
of shipping a cross-toolchain, the SDK is a **native development environment**
for the target architecture.

## Who Needs an SDK

Different developers have different needs:

| Developer type                  | What they need                            | Yoe-NG solution                  |
| ------------------------------- | ----------------------------------------- | -------------------------------- |
| Go/Rust/Zig app developer       | Native toolchain, target libs for CGO/FFI | Container with Tier 1 build root |
| Python/Node.js developer        | Runtime + native extension headers        | Container or `apk add` from repo |
| C/C++ system developer          | gcc, headers, libraries, pkg-config       | Container with full build root   |
| New team member                 | Zero-to-building setup                    | `docker run` the SDK image       |
| CI pipeline                     | Reproducible build environment            | SDK container image in registry  |
| App developer on different arch | ARM build environment on x86 laptop       | Multi-arch container via QEMU    |

For most language-native development (Go, Rust, Python), developers work on
their own machines with standard toolchains and don't need an SDK at all —
Yoe-NG just packages the output. The SDK matters when developers need **target
system libraries** (for CGO, FFI, or C/C++ development) or when the target
architecture differs from their workstation.

## SDK as a Unit

An SDK is just another unit class. It produces a development environment instead
of a package or disk image:

```python
load("//classes/sdk.star", "sdk")

sdk(
    name = "yoe-sdk",
    version = "1.0.0",
    description = "Yoe-NG development SDK for BeagleBone",
    machine = "beaglebone-black",

    # Base system packages (headers + libs + pkg-config)
    artifacts = [
        "glibc-dev",
        "linux-headers",
        "zlib-dev",
        "openssl-dev",
        "dbus-dev",
        "systemd-dev",
    ],

    # Language toolchains to include
    toolchains = ["gcc", "g++", "go", "rust", "python3"],

    # Output formats
    formats = ["container", "tarball"],
)
```

Building the SDK:

```sh
yoe build yoe-sdk
```

This produces:

- **Container image** — an OCI image with the Tier 1 build root, target
  libraries, headers, and language toolchains. Push to a registry for team use.
- **Sysroot tarball** — a `.tar.gz` of the build root for developers who prefer
  to extract it locally.

## Container-Based SDK

The container is the primary SDK format. It leverages the Tier 1 build root that
Yoe-NG already maintains — the same glibc-based environment used for building
packages, but packaged as a Docker/OCI image.

### Using the SDK

```sh
# Pull the SDK image (built by CI or a team member)
docker pull registry.example.com/yoe-sdk:latest

# Develop inside the container
docker run -it -v ./myapp:/src yoe-sdk:latest
cd /src
go build ./cmd/myapp          # native build for target arch
make -j$(nproc)               # C/C++ build against target libs
cargo build                   # Rust build
python3 -m pytest             # run tests
```

### Cross-Architecture Development

When the developer's workstation is a different architecture than the target
(e.g., x86_64 laptop targeting ARM64 board), Docker handles this transparently
via QEMU user-mode emulation:

```sh
# On x86_64 workstation, run ARM64 SDK
docker run --platform linux/arm64 -v ./myapp:/src yoe-sdk:latest
# Inside: native ARM64 environment, builds produce ARM64 binaries
```

This is slower than native hardware but works without any cross-compilation
infrastructure. For performance-critical development, use native ARM64 hardware
or cloud instances.

### Devcontainer Integration

The SDK image works directly as a VS Code devcontainer or GitHub Codespace:

```json
{
  "image": "registry.example.com/yoe-sdk:latest",
  "mounts": ["source=${localWorkspaceFolder},target=/src,type=bind"]
}
```

This gives developers a one-click setup: open the project in VS Code, it pulls
the SDK container, and they have a complete native development environment with
all target libraries available.

## Sysroot Tarball

For developers who don't want to use Docker, the SDK can be extracted as a
sysroot:

```sh
# Extract the SDK sysroot
yoe build yoe-sdk --format tarball
tar xf build/output/yoe-sdk-arm64.tar.gz -C ./sysroot

# Point build tools at the sysroot
export PKG_CONFIG_SYSROOT_DIR=./sysroot
export PKG_CONFIG_PATH=./sysroot/usr/lib/pkgconfig
pkg-config --cflags openssl   # resolves against sysroot
```

This is useful for IDE integration where Docker isn't available, or for
specialized build setups.

## Pre-Built Binary Packages

Large packages that are expensive to build from source (Chromium, Qt, LLVM,
GStreamer) don't require a special SDK mechanism. They are handled by the
standard caching infrastructure:

1. **CI builds them once** and pushes to the S3-compatible remote cache.
2. **Developers pull from cache** — `yoe build` checks the remote cache before
   building from source. If a cached `.apk` exists with the right input hash,
   it's used directly.
3. **The SDK includes them** — the SDK unit lists these packages in its
   `packages` list, so `glibc-dev`, `qt6-dev`, etc. are installed in the SDK
   container.

```python
sdk(
    name = "yoe-sdk-full",
    version = "1.0.0",
    machine = "beaglebone-black",
    artifacts = [
        "glibc-dev",
        "qt6-dev",           # pulled from cache, not built locally
        "chromium-dev",      # pulled from cache
        "gstreamer-dev",
    ],
    toolchains = ["gcc", "g++", "go"],
)
```

No developer ever needs to build Chromium from source unless they're modifying
Chromium itself. The cache ensures everyone gets the same pre-built binary.

## SDK Variants

Like image units, SDK units compose via Starlark variables:

```python
load("//classes/sdk.star", "sdk")

BASE_SDK_PACKAGES = [
    "glibc-dev",
    "linux-headers",
    "zlib-dev",
    "openssl-dev",
]

# Minimal SDK for Go microservices
sdk(
    name = "yoe-sdk-go",
    version = "1.0.0",
    machine = "beaglebone-black",
    packages = BASE_SDK_PACKAGES,
    toolchains = ["go"],
)

# Full SDK for system development
sdk(
    name = "yoe-sdk-full",
    version = "1.0.0",
    machine = "beaglebone-black",
    packages = BASE_SDK_PACKAGES + [
        "dbus-dev",
        "systemd-dev",
        "qt6-dev",
    ],
    toolchains = ["gcc", "g++", "go", "rust", "python3"],
)
```

## Workflow Summary

```
SDK maintainer (or CI):
  yoe build yoe-sdk                    ← builds SDK container + tarball
  yoe cache push yoe-sdk               ← shares with team
  docker push registry/yoe-sdk:latest  ← push container image

App developer:
  docker run -v ./myapp:/src yoe-sdk   ← native dev environment
  go build ./cmd/myapp                 ← builds for target arch
  exit
  yoe build myapp                      ← packages as .apk
  yoe build base-image                 ← includes myapp in image

New team member:
  docker run yoe-sdk                   ← instant dev environment
  # or
  code --devcontainer .                ← VS Code opens SDK container
```

The SDK is not a separate tool or workflow — it's a unit that produces a
development environment, built and cached like any other artifact.
