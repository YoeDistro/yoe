# File Templates

Move inline file content out of Starlark units into external template files
processed by Go's `text/template`. A unified `map[string]any` context serves as
both the template data and the hash input — one source of truth.

## Status: Spec

## Problem

Units currently embed multi-line file content as heredocs inside shell step
strings. This is hard to read, hard to edit, and prevents tools (syntax
highlighters, linters) from understanding the embedded content.

Examples of inline content today:

- `base-files.star` — inittab, rcS, os-release, extlinux.conf
- `network-config.star` — udhcpc default.script, S10network init script
- `image.star` — sfdisk partition tables, extlinux install scripts

## Design

### Template Files

Templates live in a directory named after the unit, alongside the `.star` file:

```
modules/units-core/
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
      network-config/
        udhcpc-default.script
        S10network
      simpleiot.star
      simpleiot/
        simpleiot.init
```

Files without `.tmpl` extension are copied verbatim via `install_file()`. Files
with `.tmpl` are processed through Go's `text/template` via
`install_template()`.

### Unit Context (`map[string]any`)

A single `map[string]any` is used for both template rendering and hash
computation. The executor auto-populates standard fields, and any extra kwargs
passed to `unit()` are captured into the same map. No separate `vars` field —
just add fields directly to the unit:

```python
unit(
    name = "my-app",
    version = "1.0.0",
    port = 8080,
    log_level = "info",
    debug = True,
    ...
)
```

Templates access all fields: `{{.port}}`, `{{.log_level}}`, `{{.name}}`.

**Auto-populated fields** (injected by the executor, not declared in the unit):

| Key       | Source                             | Example         |
| --------- | ---------------------------------- | --------------- |
| `name`    | unit name                          | `"base-files"`  |
| `version` | unit version                       | `"1.0.0"`       |
| `release` | unit release                       | `0`             |
| `arch`    | target architecture                | `"x86_64"`      |
| `machine` | active machine name                | `"qemu-x86_64"` |
| `console` | serial console from kernel cmdline | `"ttyS0"`       |
| `project` | project name                       | `"my-project"`  |

Unit kwargs override auto-populated fields if there's a name collision (explicit
wins).

**Go implementation:** `registerUnit()` captures all unrecognized kwargs into a
`map[string]any` on the Unit struct. The executor merges auto-populated fields
(lower priority) with unit fields (higher priority) to build the context map.
Classes pass `**kwargs` through to `unit()`, so custom fields flow naturally:

```python
autotools(
    name = "my-lib",
    version = "1.0",
    source = "...",
    custom_flag = "enabled",  # flows through **kwargs to unit()
)
```

### Template Syntax

Go `text/template` with the unit context map:

```
# inittab.tmpl
::sysinit:/bin/mount -t proc proc /proc
::sysinit:/bin/mount -t sysfs sys /sys
::sysinit:/bin/hostname -F /etc/hostname
::sysinit:/etc/init.d/rcS
{{.console}}::respawn:/sbin/getty -L {{.console}} 115200 vt100
::ctrlaltdel:/sbin/reboot
::shutdown:/bin/umount -a -r
```

```
# os-release.tmpl
NAME=Yoe
ID=yoe
PRETTY_NAME="Yoe Linux ({{.machine}})"
HOME_URL=https://github.com/YoeDistro/yoe
```

```
# config.toml.tmpl (custom vars)
[server]
port = {{.port}}
log_level = "{{.log_level}}"
debug = {{.debug}}
```

### Starlark API

Two new builtins callable from task functions:

```python
# install_template(src, dest, mode=0o644)
# Reads Go template from unit's files directory, renders with context, writes to dest.
install_template("inittab.tmpl", "$DESTDIR/etc/inittab")

# install_file(src, dest, mode=0o644)
# Copies file verbatim from unit's files directory to dest.
install_file("rcS", "$DESTDIR/etc/init.d/rcS", mode=0o755)
```

Paths are relative to the unit's directory (e.g., `"inittab.tmpl"` resolves to
`units/base/base-files/inittab.tmpl` for a unit defined in
`units/base/base-files.star`).

### Example: base-files with templates

**Before (inline heredocs):**

```python
task("build", steps=[
    "mkdir -p $DESTDIR/etc",
    """cat > $DESTDIR/etc/inittab << INITTAB
::sysinit:/bin/mount -t proc proc /proc
::sysinit:/bin/hostname -F /etc/hostname
${CONSOLE}::respawn:/sbin/getty -L ${CONSOLE} 115200 vt100
::ctrlaltdel:/sbin/reboot
::shutdown:/bin/umount -a -r
INITTAB""",
    """cat > $DESTDIR/etc/os-release << OSRELEASE
NAME=Yoe
ID=yoe
PRETTY_NAME="Yoe Linux ($MACHINE)"
HOME_URL=https://github.com/YoeDistro/yoe
OSRELEASE""",
])
```

**After (external templates):**

`base-files/inittab.tmpl`:

```
::sysinit:/bin/mount -t proc proc /proc
::sysinit:/bin/mount -t sysfs sys /sys
::sysinit:/bin/hostname -F /etc/hostname
::sysinit:/etc/init.d/rcS
{{.console}}::respawn:/sbin/getty -L {{.console}} 115200 vt100
::ctrlaltdel:/sbin/reboot
::shutdown:/bin/umount -a -r
```

`base-files/os-release.tmpl`:

```
NAME=Yoe
ID=yoe
PRETTY_NAME="Yoe Linux ({{.machine}})"
HOME_URL=https://github.com/YoeDistro/yoe
```

`base-files/rcS`:

```sh
#!/bin/sh
for s in /etc/init.d/S*; do
    [ -x "$s" ] && "$s" start
done
```

```python
unit(
    name = "base-files",
    version = "1.0.0",
    tasks = [
        task("build", fn=lambda: _build()),
    ],
)

def _build():
    run("mkdir -p $DESTDIR/etc $DESTDIR/root $DESTDIR/proc $DESTDIR/sys"
        + " $DESTDIR/dev $DESTDIR/tmp $DESTDIR/run")
    install_template("inittab.tmpl", "$DESTDIR/etc/inittab")
    install_template("os-release.tmpl", "$DESTDIR/etc/os-release")
    install_file("rcS", "$DESTDIR/etc/init.d/rcS", mode=0o755)
    install_template("extlinux.conf.tmpl", "$DESTDIR/boot/extlinux/extlinux.conf")
```

### Example: simpleiot init script

`simpleiot/simpleiot.init`:

```sh
#!/bin/sh
case "$1" in
    start) /usr/bin/siot &;;
    stop) killall siot;;
esac
```

```python
go_binary(
    name = "simpleiot",
    version = "0.18.5",
    services = ["simpleiot"],
    tasks = [
        task("build", steps=[...]),
        task("init-script", fn=lambda: install_file(
            "simpleiot.init", "$DESTDIR/etc/init.d/simpleiot", mode=0o755)),
    ],
)
```

### Example: custom app with extra fields

```python
unit(
    name = "my-app",
    version = "2.0.0",
    port = 8080,
    workers = 4,
    enable_tls = True,
    tasks = [
        task("config", fn=lambda: _config()),
    ],
)

def _config():
    install_template("app.conf.tmpl", "$DESTDIR/etc/my-app/app.conf")
```

`my-app/app.conf.tmpl`:

```
# Generated by Yoe for {{.machine}}
listen_port = {{.port}}
workers = {{.workers}}
{{if .enable_tls}}tls_cert = /etc/ssl/certs/ca-certificates.crt{{end}}
```

### Hashing

The unit context map (`map[string]any`) is JSON-serialized with sorted keys and
included in the unit hash. This means:

- Changing any unit field changes the hash and triggers a rebuild
- Auto-populated fields (arch, machine) already affect the hash through existing
  mechanisms, but including them in the context map makes it explicit
- No separate hash logic needed for template fields vs build fields

Additionally, all files in the unit's files directory
(`<DefinedIn>/<unit-name>/`) are hashed by content. Changing a template file
changes the hash.

### Path Resolution

Template paths resolve to `<DefinedIn>/<unit-name>/<relPath>`:

```go
func resolveTemplatePath(unit *Unit, relPath string) string {
    return filepath.Join(unit.DefinedIn, unit.Name, relPath)
}
```

This matches the existing container convention:

| Unit file                        | Associated directory         |
| -------------------------------- | ---------------------------- |
| `containers/toolchain-musl.star` | `containers/toolchain-musl/` |
| `units/base/base-files.star`     | `units/base/base-files/`     |
| `units/net/network-config.star`  | `units/net/network-config/`  |

### Go Implementation

**New file: `internal/build/templates.go`**

- `TemplateContext` — carries `map[string]any` context
- `fnInstallTemplate` — Starlark builtin: read template, render, write
- `fnInstallFile` — Starlark builtin: copy file verbatim
- `resolveTemplatePath` — resolve `<DefinedIn>/<unit-name>/<relPath>`
- `expandEnv` — expand `$DESTDIR` etc. in destination paths
- `templateFuncs` — custom Go template functions (`sizeMB`, `sfdiskType`)

**Modified: `internal/build/executor.go`**

- Build the `map[string]any` context from unit fields, machine, and project
- Pass context to build thread via thread-local storage

**Modified: `internal/build/starlark_exec.go`**

- Store template context on build thread
- Register `install_template` and `install_file` builtins

**Modified: `internal/starlark/builtins.go`**

- Add `install_template` and `install_file` placeholders (same pattern as
  `run()`)
- Capture unrecognized kwargs in `registerUnit()` into `Extra map[string]any`
  on the Unit struct

**Modified: `internal/starlark/types.go`**

- Add `Extra map[string]any` field to Unit struct for unrecognized kwargs

**Modified: `internal/resolve/hash.go`**

- JSON-serialize the context map and include in hash
- Hash contents of all files in unit's files directory

### What Stays in Go

Template rendering runs on the host (Go executor), not in the container. This
keeps template data (machine config, unit metadata) accessible without passing
it through environment variables. The rendered files are placed in the build
directory, then the container mounts them.

## Implementation Order

1. **`Extra` field on Unit** — capture unrecognized kwargs in `registerUnit()`
2. **`install_file()` builtin** — copy static files
3. **`install_template()` builtin** — Go template rendering with context map
4. **Context map and hashing** — build context from unit fields + extra +
   machine/project, hash it and unit files directory
5. **Migrate base-files** — move inittab, rcS, os-release, extlinux.conf to
   template files
6. **Migrate network-config** — move udhcpc script and init script to files
7. **Migrate simpleiot** — move init script to file

## Non-Goals

- **Jinja2 or other template engines.** Go `text/template` is in stdlib and
  sufficient.
- **Template inheritance or includes.** Keep templates flat and simple.
- **Build-time template rendering in the container.** Templates are rendered by
  the Go executor on the host.
