# Per-Recipe Container and Task Support

## Context

All yoe-ng recipes currently build inside a single Alpine container using bwrap
for per-recipe isolation, with build steps as a flat list of shell commands.
Different recipes need different toolchains — Go apps need the Go SDK, Rust apps
need Cargo, kernel builds need specific headers. And some recipes have distinct
phases that benefit from different environments (codegen in one container,
compile in another).

## Design

Two new concepts:

1. **`container` field** — optional Docker image at the package or task level.
   When set, that step runs inside the specified container instead of bwrap.
2. **`task()` function** — named build steps that replace the `build = [...]`
   string list. Each task can specify its own container.

### Resolution Order

For each task, the execution environment is determined by:

1. Task `container` if set → runs in that container
2. Package `container` if set → runs in that container
3. Neither set → runs in bwrap (default)

### Backward Compatibility

The `build = [...]` string list continues to work. Internally it is converted to
a list of unnamed tasks without containers. Existing recipes need no changes.

## Usage Examples

```python
# Simple — build list works as before (bwrap)
autotools(name = "zlib", source = "...", ...)

# Package-level container — all tasks inherit it
go_binary(
    name = "myapp",
    container = "golang:1.22-alpine",
    tasks = [
        task("build", run="go build -o $DESTDIR/usr/bin/myapp"),
        task("test", run="go test ./..."),
    ],
)

# Task-level override — codegen uses a different container
package(
    name = "complex-app",
    container = "golang:1.22-alpine",       # default for all tasks
    tasks = [
        task("codegen",
             container="protoc:latest",     # overrides package default
             run="protoc --go_out=. api/*.proto"),
        task("compile",
             run="go build -o $DESTDIR/usr/bin/app"),  # uses package default
        task("install",
             run="install -D app.service $DESTDIR/usr/lib/systemd/system/"),
    ],
)

# Mix of container and bwrap tasks
package(
    name = "hybrid-tool",
    tasks = [
        task("generate",
             container="codegen-tools:latest",
             run="generate-code --out src/"),
        task("compile",                     # no container → bwrap
             run="make -j$NPROC"),
        task("install",
             run="make DESTDIR=$DESTDIR install"),
    ],
)

# Class sets default container, user overrides
go_binary(
    name = "myapp",
    source = "https://github.com/example/myapp.git",
    container = "golang:1.23-alpine",       # override class default of 1.22
)
```

### How Classes Generate Tasks

Classes produce task lists instead of build step strings:

```python
# classes/autotools.star
def autotools(name, version, source, configure_args=[], **kwargs):
    package(
        name=name, version=version, source=source,
        tasks = [
            task("configure",
                 run="test -f configure || autoreconf -fi && "
                     "./configure --prefix=$PREFIX " + " ".join(configure_args)),
            task("compile", run="make -j$NPROC"),
            task("install", run="make DESTDIR=$DESTDIR install"),
        ],
        **kwargs,
    )

# classes/go.star
def go_binary(name, version, source, go_package="",
              container="golang:1.22-alpine", **kwargs):
    if not go_package:
        go_package = "./cmd/" + name
    package(
        name=name, version=version, source=source,
        container=container,
        tasks = [
            task("build",
                 run="go build -o $DESTDIR$PREFIX/bin/" + name + " " + go_package),
        ],
        **kwargs,
    )
```

## Files to Modify

### 1. `internal/starlark/types.go`

Add `Task` struct and update `Recipe`:

```go
// Task represents a named build step with optional container override.
type Task struct {
    Name      string
    Container string // overrides recipe Container if set
    Run       string // shell command
}

// In Recipe struct:
type Recipe struct {
    // ... existing fields ...

    // Build — either Tasks or Build (legacy), not both
    Container string // default container for all tasks (empty = bwrap)
    Tasks     []Task // named build steps with optional per-task containers
    Build     []string // legacy: flat list of shell commands (converted to Tasks)

    // ... rest of fields ...
}
```

### 2. `internal/starlark/builtins.go`

- Add `task()` builtin function returning a struct
- Parse `container` kwarg in `registerRecipe()`
- Parse `tasks` kwarg as list of task structs
- If `tasks` is empty but `build` is set, convert `build` strings to unnamed
  tasks (backward compat)

### 3. `internal/resolve/hash.go`

Include container and tasks in hash:

```go
fmt.Fprintf(h, "container:%s\n", recipe.Container)
for _, t := range recipe.Tasks {
    fmt.Fprintf(h, "task:%s:%s:%s\n", t.Name, t.Container, t.Run)
}
```

### 4. `internal/build/sandbox.go`

Add `ContainerConfig` and `RunInContainer()`:

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

Runs:
`docker run --rm -v src:/build/src -v dest:/build/destdir -v sysroot:/build/sysroot:ro -w /build/src -e KEY=VALUE... <image> sh -c <cmd>`

### 5. `internal/build/executor.go`

Replace the command loop with a task loop. For each task, resolve the container:

```go
tasks := recipe.Tasks
if len(tasks) == 0 {
    // Legacy: convert build strings to tasks
    for i, cmd := range buildCommands(recipe) {
        tasks = append(tasks, Task{Name: fmt.Sprintf("step-%d", i+1), Run: cmd})
    }
}

for i, t := range tasks {
    container := t.Container
    if container == "" {
        container = recipe.Container
    }

    fmt.Fprintf(w, "  [%d/%d] %s: %s\n", i+1, len(tasks), t.Name, t.Run)

    if container != "" {
        RunInContainer(&ContainerConfig{Image: container, ...}, t.Run)
    } else if opts.UseSandbox && HasBwrap() {
        RunInSandbox(cfg, t.Run)
    } else {
        RunSimple(srcDir, destDir, env, t.Run)
    }
}
```

### 6. `internal/container.go`

Mount Docker socket for docker-in-docker:

```go
if _, err := os.Stat("/var/run/docker.sock"); err == nil {
    runArgs = append(runArgs, "-v", "/var/run/docker.sock:/var/run/docker.sock")
}
```

Bump `containerVersion`.

### 7. `containers/Dockerfile.build`

Add `docker-cli` to `apk add` list. Bump version.

### 8. Layer classes

Update `go.star` to use tasks and set default container. Update `autotools.star`
and `cmake.star` to generate tasks instead of build strings (backward compat
maintained through the legacy conversion in the executor).

## Tradeoffs

| Aspect          | bwrap (default)      | Per-task container |
| --------------- | -------------------- | ------------------ |
| Startup         | ~1ms                 | ~200ms             |
| Isolation       | Namespace only       | Full container     |
| Toolchain       | Shared (Alpine's)    | Per-task, pinned   |
| Reproducibility | Depends on container | Pinned per task    |
| Sysroot         | Mounted in           | Mounted in (same)  |

## Verification

1. `go test ./...` — all existing tests pass
2. Legacy `build = [...]` recipes still work (backward compat)
3. Go recipe with task container pulls golang image and builds
4. Recipe with mixed tasks (container + bwrap) works
5. Package-level container inherited by tasks without override
6. Task-level container overrides package-level
7. Hash changes when container or task fields change
8. Docker socket mount works: `docker ps` inside yoe-ng container
