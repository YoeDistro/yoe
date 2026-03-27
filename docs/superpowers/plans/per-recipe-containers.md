# Per-Recipe Container Support

## Context

All yoe-ng recipes currently build inside a single Alpine container using bwrap
for per-recipe isolation. Different recipes need different toolchains — Go apps
need the Go SDK, Rust apps need Cargo, kernel builds need specific headers.
Adding an optional `container` field to recipes allows each recipe to specify
its own Docker image, while recipes without it continue using bwrap (fast,
simple).

## Approach

Optional `container` field on recipes. When set, the executor runs `docker run`
with that image instead of bwrap. The sysroot, source, and destdir mounts are
identical either way. Classes can set defaults (e.g., `go.star` defaults to
`golang:1.22-alpine`). Users can override at recipe level.

Docker-in-docker: mount the host Docker socket into the yoe-ng container and add
`docker-cli` to the Dockerfile.

## Files to Modify

### 1. `internal/starlark/types.go`

Add `Container string` to Recipe struct (in the `// Build` section):

```go
Container     string   // Docker image for build (empty = use bwrap)
```

### 2. `internal/starlark/builtins.go`

In `registerRecipe()`, add:

```go
Container:     kwString(kwargs, "container"),
```

### 3. `internal/resolve/hash.go`

In `RecipeHash()`, add after build config section:

```go
fmt.Fprintf(h, "container:%s\n", recipe.Container)
```

### 4. `internal/build/sandbox.go`

Add new struct and function:

```go
type ContainerConfig struct {
    Image   string
    SrcDir  string
    DestDir string
    Sysroot string
    Env     map[string]string
}

func RunInContainer(cfg *ContainerConfig, command string) error
```

`RunInContainer` builds:
`docker run --rm -v src:/build/src -v dest:/build/destdir -v sysroot:/build/sysroot:ro -w /build/src -e KEY=VALUE... <image> sh -c <cmd>`

Also add a local `detectRuntime()` (5 lines — check docker then podman in PATH).

### 5. `internal/build/executor.go`

Three-way dispatch in `buildOne()`:

```go
if recipe.Container != "" {
    RunInContainer(...)    // per-recipe container
} else if opts.UseSandbox && HasBwrap() {
    RunInSandbox(...)      // bwrap (current default)
} else {
    RunSimple(...)         // fallback
}
```

Container path takes priority — recipe author explicitly chose an image.

### 6. `internal/container.go`

Mount Docker socket for docker-in-docker:

```go
if _, err := os.Stat("/var/run/docker.sock"); err == nil {
    runArgs = append(runArgs, "-v", "/var/run/docker.sock:/var/run/docker.sock")
}
```

Bump `containerVersion`.

### 7. `containers/Dockerfile.build`

Add `docker-cli` to `apk add` list. Bump version comment.

### 8. `layers/recipes-core/classes/go.star`

Add `container` parameter with default:

```python
def go_binary(..., container="golang:1.22-alpine", **kwargs):
    package(..., container=container, **kwargs)
```

Other classes (autotools, cmake) don't need changes — `**kwargs` passthrough
already forwards any `container=` kwarg from recipe level.

## Usage Examples

```python
# Class sets default container
go_binary(
    name = "myapp",
    source = "https://github.com/example/myapp.git",
    tag = "v1.0",
)
# → builds inside golang:1.22-alpine

# Override at recipe level
go_binary(
    name = "myapp",
    source = "https://github.com/example/myapp.git",
    container = "golang:1.23-alpine",
)

# Any recipe can specify a container
package(
    name = "custom-tool",
    container = "rust:1.78-alpine",
    build = [
        "cargo build --release",
        "install -D target/release/tool $DESTDIR/usr/bin/tool",
    ],
)

# C/C++ recipes use bwrap (no container field = current behavior)
autotools(name = "zlib", ...)
```

## Tradeoffs

| Aspect          | bwrap (default)      | Per-recipe container |
| --------------- | -------------------- | -------------------- |
| Startup         | ~1ms                 | ~200ms               |
| Isolation       | Namespace only       | Full container       |
| Toolchain       | Shared (Alpine's)    | Per-recipe, pinned   |
| Reproducibility | Depends on container | Pinned per recipe    |
| Sysroot         | Mounted in           | Mounted in (same)    |

## Verification

1. `go test ./...` — all existing tests pass
2. Build a Go recipe with container: `yoe build myapp` — pulls golang image
3. Build a C recipe without container: `yoe build zlib` — uses bwrap as before
4. Verify hash changes: different `container` values produce different hashes
5. Verify docker socket mount: `docker ps` works inside the yoe-ng container
