# Per-Unit Sysroots

## Context

The current shared sysroot (`build/sysroot/`) is populated by every non-image
unit after it builds. This causes collisions: busybox installs hundreds of
symlinks to `/bin/` and `/usr/bin/` that shadow container tools. Specifically,
the sysroot's busybox binary is musl-linked but targets the final image, not the
build container. When a unit like util-linux builds, autoconf's `expr` calls
resolve to the sysroot's broken busybox symlink instead of the container's GNU
coreutils `expr`, causing an infinite loop and fd exhaustion.

The root cause is architectural: the sysroot should only contain artifacts from
a unit's declared `deps`, not from every previously-built unit.

## Design

**Per-unit sysroots populated only from declared `deps`.**

Instead of one shared `build/sysroot/`, each unit gets its own sysroot assembled
from only its dependency chain. The DAG already knows each unit's deps and the
topological order guarantees deps are built before dependents.

### How it works

1. After a unit builds, its destdir contents are stored in a per-unit staging
   area: `build/<unit>/sysroot-stage/` (a copy or hardlink of destdir).

2. Before building a unit, its sysroot is assembled by merging the sysroot-stage
   directories of all transitive `deps` (not `runtime_deps`, not image
   `artifacts`) into `build/<unit>/sysroot/`.

3. The sandbox mounts this per-unit sysroot at `/build/sysroot` (read-only, same
   as today). No env var changes needed.

4. The shared `build/sysroot/` directory is removed.

### Example

```
util-linux deps: [ncurses, zlib]

build/util-linux/sysroot/ contains:
  - ncurses destdir contents (headers, libs, terminfo)
  - zlib destdir contents (headers, libs)
  - nothing else (no busybox, no openssh)
```

### Transitive deps

If A depends on B which depends on C, A's sysroot contains B + C. This is
already computable from the DAG via the dependency graph. The resolver's
`dag.go` has `Node.Deps` for direct deps; transitive closure is a simple graph
walk.

## Files to modify

| File                         | Change                                                                    |
| ---------------------------- | ------------------------------------------------------------------------- |
| `internal/build/executor.go` | Replace shared sysroot with per-unit sysroot assembly                     |
| `internal/build/sandbox.go`  | Remove `InstallToSysroot()`, add `AssembleSysroot()` and `StageSysroot()` |
| `internal/resolve/dag.go`    | Add `TransitiveDeps(name)` helper to compute full dep closure             |

### executor.go changes

In `buildOne()`:

- **Before building**: call `AssembleSysroot()` to merge transitive deps' staged
  outputs into `build/<unit>/sysroot/`
- **After building**: call `StageSysroot()` to copy destdir to
  `build/<unit>/sysroot-stage/` (replacing the current `InstallToSysroot` call)
- Pass `build/<unit>/sysroot` as the sysroot path in `SandboxConfig` instead of
  the shared `build/sysroot`

```go
// Before build: assemble sysroot from deps
recipeSysroot := filepath.Join(buildDir, "sysroot")
if err := AssembleSysroot(recipeSysroot, dag, name, projectDir); err != nil {
    return fmt.Errorf("assembling sysroot: %w", err)
}

// ... build commands ...

// After build: stage destdir for downstream units
if err := StageSysroot(destDir, buildDir); err != nil {
    return fmt.Errorf("staging sysroot: %w", err)
}
```

### sandbox.go changes

Remove `InstallToSysroot()`. Add:

```go
// StageSysroot copies a unit's destdir to its sysroot staging area
// so downstream units can include it in their sysroots.
func StageSysroot(destDir, buildDir string) error {
    stageDir := filepath.Join(buildDir, "sysroot-stage")
    os.RemoveAll(stageDir)
    os.MkdirAll(stageDir, 0755)
    return exec.Command("cp", "-a", destDir+"/.", stageDir+"/").Run()
}

// AssembleSysroot merges the sysroot-stage dirs of all transitive deps
// into a unit's private sysroot.
func AssembleSysroot(sysrootDir string, dag *resolve.DAG, unit string, projectDir string) error {
    os.RemoveAll(sysrootDir)
    os.MkdirAll(sysrootDir, 0755)
    for _, dep := range dag.TransitiveDeps(unit) {
        stageDir := filepath.Join(RecipeBuildDir(projectDir, dep), "sysroot-stage")
        if _, err := os.Stat(stageDir); err != nil {
            continue // dep has no staged output (e.g., image)
        }
        exec.Command("cp", "-a", stageDir+"/.", sysrootDir+"/").Run()
    }
    return nil
}
```

### dag.go changes

Add a `TransitiveDeps` method:

```go
// TransitiveDeps returns all transitive dependencies of a node
// in topological order (deepest deps first).
func (d *DAG) TransitiveDeps(name string) []string {
    visited := map[string]bool{}
    var result []string
    var walk func(n string)
    walk = func(n string) {
        if visited[n] { return }
        visited[n] = true
        if node, ok := d.Nodes[n]; ok {
            for _, dep := range node.Deps {
                walk(dep)
            }
        }
        result = append(result, n)
    }
    // Walk deps of the target, not the target itself
    if node, ok := d.Nodes[name]; ok {
        for _, dep := range node.Deps {
            walk(dep)
        }
    }
    return result
}
```

## What this removes

- The `NoSysroot` field on `Unit` (no longer needed)
- The shared `build/sysroot/` directory
- `InstallToSysroot()` function
- `/build/sysroot/usr/bin` on PATH (each unit's sysroot only has its deps)

## Cache implications

- The content-addressed hash already includes dependency hashes, so cache
  invalidation is unchanged.
- The per-unit sysroot is rebuilt from staged dirs on each build; it's not
  cached independently.
- `sysroot-stage/` is part of the build dir and persists across builds (same
  lifecycle as destdir).

## Performance consideration

Assembling per-unit sysroots via `cp -a` for each unit adds I/O. For units with
many transitive deps, this could be noticeable. Mitigations (implement later if
needed):

- Use hardlinks (`cp -al`) instead of copies (same filesystem)
- Use overlayfs to layer deps (avoids copying entirely)

Start with `cp -a` for correctness; optimize if profiling shows it matters.

## Verification

1. `source envsetup.sh && yoe_test` — all existing tests pass
2. Clean build from e2e-project:
   ```
   cd testdata/e2e-project
   rm -rf build
   yoe build base-image
   ```
3. Verify no shared sysroot exists: `ls build/sysroot` should fail
4. Verify per-unit sysroots: `ls build/util-linux/sysroot/` should contain only
   ncurses + zlib artifacts
5. Verify busybox symlinks don't appear in other units' sysroots
6. `yoe build --force util-linux` succeeds (the original bug)
