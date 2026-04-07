# File Templates

Move inline file content out of Starlark units into external template files
processed by Go's `text/template`.

## Status: Spec

## Problem

Units currently embed multi-line file content as heredocs inside shell step
strings. This is hard to read, hard to edit, and prevents tools (syntax
highlighters, linters) from understanding the embedded content.

Examples of inline content today:

- `base-files.star` — inittab, rcS, os-release, extlinux.conf (lines 49-85)
- `network-config.star` — udhcpc default.script, S10network init script (lines
  14-42)
- `image.star` — sfdisk partition tables, extlinux install scripts (lines
  74-138)

A typical unit mixes build logic with file content:

```python
# Hard to read, no syntax highlighting, escaping issues
"""cat > $DESTDIR/etc/inittab << INITTAB
::sysinit:/bin/mount -t proc proc /proc
::sysinit:/bin/hostname -F /etc/hostname
${CONSOLE}::respawn:/sbin/getty -L ${CONSOLE} 115200 vt100
INITTAB"""
```

## Design

### Template Files

Templates live in a directory named after the unit, alongside the `.star` file.
This matches the existing container convention where
`containers/toolchain-musl.star` has its Dockerfile in
`containers/toolchain-musl/Dockerfile`:

```
modules/units-core/
  containers/
    toolchain-musl.star          # existing pattern
    toolchain-musl/
      Dockerfile
  units/
    base/
      base-files.star
      base-files/                # same name as the unit
        inittab.tmpl
        rcS
        os-release.tmpl
        extlinux.conf.tmpl
    net/
      network-config.star
      network-config/            # same name as the unit
        udhcpc-default.script
        S10network
  classes/
    image.star
    image/
      sfdisk.tmpl
      fstab.tmpl
```

Files without `.tmpl` extension are copied verbatim. Files with `.tmpl` are
processed through Go's `text/template` engine.

### Template Syntax

Go `text/template` with unit/machine/project data:

```
# inittab.tmpl
::sysinit:/bin/mount -t proc proc /proc
::sysinit:/bin/mount -t sysfs sys /sys
::sysinit:/bin/hostname -F /etc/hostname
::sysinit:/etc/init.d/rcS
{{.Console}}::respawn:/sbin/getty -L {{.Console}} 115200 vt100
::ctrlaltdel:/sbin/reboot
::shutdown:/bin/umount -a -r
```

```
# os-release.tmpl
NAME=Yoe
ID=yoe
PRETTY_NAME="Yoe Linux ({{.Machine}})"
HOME_URL=https://github.com/YoeDistro/yoe
```

```
# extlinux.conf.tmpl
DEFAULT yoe
LABEL yoe
    LINUX /boot/vmlinuz
    APPEND {{.KernelCmdline}}
```

### Starlark API

A new `install_template()` builtin and `install_file()` builtin, callable from
task steps:

```python
# base-files.star
unit(
    name = "base-files",
    version = "1.0.0",
    tasks = [
        task("build", fn = lambda: _build()),
    ],
)

def _build():
    run("mkdir -p $DESTDIR/etc $DESTDIR/root $DESTDIR/proc $DESTDIR/sys $DESTDIR/dev $DESTDIR/tmp $DESTDIR/run")

    # Template — rendered with unit/machine data
    install_template("inittab.tmpl", "$DESTDIR/etc/inittab")
    install_template("os-release.tmpl", "$DESTDIR/etc/os-release")

    # Static file — copied verbatim
    install_file("rcS", "$DESTDIR/etc/init.d/rcS", mode = 0o755)

    # Boot config — only if machine uses extlinux
    install_template("extlinux.conf.tmpl", "$DESTDIR/boot/extlinux/extlinux.conf")
```

Paths are relative to the unit's directory (e.g., `"inittab.tmpl"` resolves to
`units/base/base-files/inittab.tmpl` for a unit defined in
`units/base/base-files.star`).

### Template Data

The template engine receives a data map built from the unit, machine, and
project context:

```go
data := TemplateData{
    // Unit fields
    Name:    unit.Name,
    Version: unit.Version,

    // Machine fields (from project's active machine)
    Machine:       machine.Name,
    Arch:          machine.Arch,
    Console:       extractConsole(machine.Kernel.Cmdline),
    KernelCmdline: machine.Kernel.Cmdline,

    // Project fields
    Project: proj.Name,

    // Partitions (for disk layout templates)
    Partitions: machine.Partitions,

    // Custom key-value pairs from unit
    Vars: unit.Vars,
}
```

Units can pass additional variables via a `vars` field:

```python
unit(
    name = "my-app",
    vars = {"port": "8080", "log_level": "info"},
    ...
)
```

Accessible in templates as `{{.Vars.port}}`.

### Path Resolution

Template paths are resolved to a directory named after the unit, alongside the
`.star` file. This uses the existing `DefinedIn` field (directory containing the
`.star` file) plus the unit name:

```go
func resolveTemplatePath(unit *Unit, relPath string) string {
    return filepath.Join(unit.DefinedIn, unit.Name, relPath)
}
```

This means `install_template("inittab.tmpl", ...)` in a unit defined at
`modules/units-core/units/base/base-files.star` resolves to
`modules/units-core/units/base/base-files/inittab.tmpl`.

This matches the existing container convention:

| Unit file                        | Associated directory         |
| -------------------------------- | ---------------------------- |
| `containers/toolchain-musl.star` | `containers/toolchain-musl/` |
| `units/base/base-files.star`     | `units/base/base-files/`     |
| `units/net/network-config.star`  | `units/net/network-config/`  |
| `classes/image.star`             | `classes/image/`             |

### Go Implementation

Two new Starlark builtins registered in the engine:

```go
// install_template(src, dest, mode=0644)
// - Reads template from src (relative to unit's DefinedIn)
// - Renders with template data
// - Writes to dest (environment variables expanded)
// - Sets file mode
func (e *Engine) builtinInstallTemplate(kwargs []starlark.Tuple) (starlark.Value, error)

// install_file(src, dest, mode=0644)
// - Copies file from src (relative to unit's DefinedIn)
// - Writes to dest (environment variables expanded)
// - Sets file mode
func (e *Engine) builtinInstallFile(kwargs []starlark.Tuple) (starlark.Value, error)
```

Both builtins:

1. Resolve the source path to `<DefinedIn>/<unit-name>/<src>`
2. Expand environment variables in the destination path (`$DESTDIR`, etc.)
3. Create parent directories as needed
4. Write with the specified mode (default 0644)

`install_template` additionally parses the source as a Go template and executes
it with the template data map.

### Conditionals and Loops in Templates

Go templates support conditionals and iteration, useful for generated config:

```
# fstab.tmpl
{{range .Partitions -}}
LABEL={{.Label}}  {{if .Root}}/{{else}}/mnt/{{.Label}}{{end}}  {{.Type}}  defaults  0  {{if .Root}}1{{else}}0{{end}}
{{end -}}
```

```
# sfdisk.tmpl
label: dos
{{range $i, $p := .Partitions -}}
{{if not (isLast $i $.Partitions)}}size={{sizeMB $p.Size}}MiB, {{end}}type={{sfdiskType $p.Type}}{{if $p.Root}}, bootable{{end}}
{{end -}}
```

Custom template functions registered for disk operations:

| Function     | Purpose                           |
| ------------ | --------------------------------- |
| `sizeMB`     | Parse "256M", "1G" to integer MB  |
| `sfdiskType` | Map "ext4" to "83", "vfat" to "c" |
| `isLast`     | Check if index is last element    |

### Image Class with Templates

The image class moves disk layout generation to templates:

```python
# classes/image.star
def _create_disk_image(name, partitions):
    if not partitions:
        return
    total_mb = 1
    for p in partitions:
        total_mb += _parse_size_mb(p.size)

    img = "$DESTDIR/%s.img" % name
    run("dd if=/dev/zero of=%s bs=1M count=0 seek=%d" % (img, total_mb))

    # Partition table from template instead of inline printf
    install_template("sfdisk.tmpl", "$DESTDIR/sfdisk.script")
    run("sfdisk %s < $DESTDIR/sfdisk.script" % img)
    # ... rest of disk assembly
```

## Migration

### Before (inline heredoc)

```python
task("build", steps=[
    """cat > $DESTDIR/etc/inittab << INITTAB
::sysinit:/bin/mount -t proc proc /proc
::sysinit:/bin/hostname -F /etc/hostname
${CONSOLE}::respawn:/sbin/getty -L ${CONSOLE} 115200 vt100
::ctrlaltdel:/sbin/reboot
::shutdown:/bin/umount -a -r
INITTAB""",
])
```

### After (external template)

`base-files/inittab.tmpl`:

```
::sysinit:/bin/mount -t proc proc /proc
::sysinit:/bin/mount -t sysfs sys /sys
::sysinit:/bin/hostname -F /etc/hostname
::sysinit:/etc/init.d/rcS
{{.Console}}::respawn:/sbin/getty -L {{.Console}} 115200 vt100
::ctrlaltdel:/sbin/reboot
::shutdown:/bin/umount -a -r
```

```python
task("build", fn = lambda: _build()),

def _build():
    run("mkdir -p $DESTDIR/etc")
    install_template("inittab.tmpl", "$DESTDIR/etc/inittab")
```

### Files to Migrate

| Unit file             | Inline content         | Template file                                   |
| --------------------- | ---------------------- | ----------------------------------------------- |
| `base-files.star`     | inittab                | `base-files/inittab.tmpl`                       |
| `base-files.star`     | rcS                    | `base-files/rcS` (static)                       |
| `base-files.star`     | os-release             | `base-files/os-release.tmpl`                    |
| `base-files.star`     | extlinux.conf          | `base-files/extlinux.conf.tmpl`                 |
| `network-config.star` | udhcpc default.script  | `network-config/udhcpc-default.script` (static) |
| `network-config.star` | S10network             | `network-config/S10network` (static)            |
| `image.star`          | sfdisk partition table | `image/sfdisk.tmpl`                             |

Note: static files (no variables) use `install_file()` — no template processing,
just a clean copy. This avoids accidental template interpretation of file
content that happens to contain `{{`.

## Implementation Order

1. **`install_file()` builtin** — copy static files. Migrate rcS, udhcpc
   default.script, S10network.
2. **`install_template()` builtin** — Go template rendering with unit/machine
   data. Migrate inittab, os-release, extlinux.conf.
3. **Template functions** — `sizeMB`, `sfdiskType`, `isLast` for disk layout
   templates.
4. **`vars` field on unit** — custom key-value pairs accessible in templates.
5. **Migrate image class** — sfdisk partition table as template.

## Non-Goals

- **Jinja2 or other template engines.** Go `text/template` is in stdlib and
  sufficient.
- **Template inheritance or includes.** Keep templates flat and simple. If a
  template needs to compose, use multiple `install_template()` calls.
- **Build-time template rendering in the container.** Templates are rendered by
  the Go executor on the host before files are placed in the build directory.
  This keeps template data (machine config, unit metadata) accessible without
  passing it through environment variables.
