package starlark

import (
	"fmt"
	"path/filepath"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// builtins returns the predeclared names available in all .star files.
func (e *Engine) builtins() starlark.StringDict {
	d := starlark.StringDict{
		"project":     starlark.NewBuiltin("project", e.fnProject),
		"defaults":    starlark.NewBuiltin("defaults", fnDefaults),
		"cache":       starlark.NewBuiltin("cache", fnCache),
		"s3_cache":    starlark.NewBuiltin("s3_cache", fnS3Cache),
		"sources":     starlark.NewBuiltin("sources", fnSources),
		"module":      starlark.NewBuiltin("module", fnModule),
		"module_info": starlark.NewBuiltin("module_info", e.fnModuleInfo),
		"machine":     starlark.NewBuiltin("machine", e.fnMachine),
		"kernel":      starlark.NewBuiltin("kernel", fnKernel),
		"uboot":       starlark.NewBuiltin("uboot", fnUboot),
		"qemu_config": starlark.NewBuiltin("qemu_config", fnQEMUConfig),
		"unit":        starlark.NewBuiltin("unit", e.fnUnit),
		"image":       starlark.NewBuiltin("image", e.fnImage),
		"partition":   starlark.NewBuiltin("partition", fnPartition),
		"task":        starlark.NewBuiltin("task", fnTask),
		"command":     starlark.NewBuiltin("command", e.fnCommand),
		"arg":         starlark.NewBuiltin("arg", fnArg),
		"run":            starlark.NewBuiltin("run", fnRunPlaceholder),
		"apk_create":    starlark.NewBuiltin("apk_create", fnBuildtimePlaceholder("apk_create")),
		"apk_publish":   starlark.NewBuiltin("apk_publish", fnBuildtimePlaceholder("apk_publish")),
		"sysroot_stage": starlark.NewBuiltin("sysroot_stage", fnBuildtimePlaceholder("sysroot_stage")),
		"True":        starlark.True,
		"False":       starlark.False,
	}

	// Merge engine variables (e.g., ARCH set after machine loading).
	for k, v := range e.vars {
		d[k] = v
	}

	return d
}

// fnRunPlaceholder is registered as a global so that lambda closures can
// capture the name "run" at evaluation time.  When called from a build
// thread (with sandbox config in thread-local storage) it delegates to the
// real implementation in the build package.  When called outside a build
// thread it returns an error.
func fnRunPlaceholder(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Check if we're in a build thread by looking for the sandbox key.
	if thread.Local("yoe.sandbox") != nil {
		// Delegate to the real run() registered via BuildPredeclared.
		// The build package sets "yoe.run" on the thread.
		if fn := thread.Local("yoe.run"); fn != nil {
			if callable, ok := fn.(starlark.Callable); ok {
				return starlark.Call(thread, callable, args, kwargs)
			}
		}
	}
	return nil, fmt.Errorf("run() can only be called at build time (inside a task function)")
}

// fnBuildtimePlaceholder creates a placeholder for build-time builtins that
// delegates to the real implementation stored on the build thread.
func fnBuildtimePlaceholder(name string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	key := "yoe." + name
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if thread.Local("yoe.sandbox") != nil {
			if fn := thread.Local(key); fn != nil {
				if callable, ok := fn.(starlark.Callable); ok {
					return starlark.Call(thread, callable, args, kwargs)
				}
			}
		}
		return nil, fmt.Errorf("%s() can only be called at build time (inside a task function)", name)
	}
}

// --- Helper: extract keyword args ---

func kwString(kwargs []starlark.Tuple, key string) string {
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == key {
			if s, ok := kv[1].(starlark.String); ok {
				return string(s)
			}
		}
	}
	return ""
}

func kwInt(kwargs []starlark.Tuple, key string) int {
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == key {
			if n, ok := kv[1].(starlark.Int); ok {
				v, _ := n.Int64()
				return int(v)
			}
		}
	}
	return 0
}

func kwStringList(kwargs []starlark.Tuple, key string) []string {
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == key {
			if list, ok := kv[1].(*starlark.List); ok {
				var result []string
				iter := list.Iterate()
				defer iter.Done()
				var v starlark.Value
				for iter.Next(&v) {
					if s, ok := v.(starlark.String); ok {
						result = append(result, string(s))
					}
				}
				return result
			}
		}
	}
	return nil
}

func kwStruct(kwargs []starlark.Tuple, key string) *starlarkstruct.Struct {
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == key {
			if s, ok := kv[1].(*starlarkstruct.Struct); ok {
				return s
			}
		}
	}
	return nil
}

func structString(s *starlarkstruct.Struct, field string) string {
	if s == nil {
		return ""
	}
	v, err := s.Attr(field)
	if err != nil {
		return ""
	}
	if str, ok := v.(starlark.String); ok {
		return string(str)
	}
	return ""
}

func structStringList(s *starlarkstruct.Struct, field string) []string {
	if s == nil {
		return nil
	}
	v, err := s.Attr(field)
	if err != nil {
		return nil
	}
	if list, ok := v.(*starlark.List); ok {
		var result []string
		iter := list.Iterate()
		defer iter.Done()
		var item starlark.Value
		for iter.Next(&item) {
			if str, ok := item.(starlark.String); ok {
				result = append(result, string(str))
			}
		}
		return result
	}
	return nil
}

// --- Built-in functions that return structs (data constructors) ---

func makeStruct(name string, kwargs []starlark.Tuple) *starlarkstruct.Struct {
	d := make(starlark.StringDict, len(kwargs))
	for _, kv := range kwargs {
		d[string(kv[0].(starlark.String))] = kv[1]
	}
	return starlarkstruct.FromStringDict(starlark.String(name), d)
}

func fnDefaults(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return makeStruct("defaults", kwargs), nil
}

func fnCache(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return makeStruct("cache", kwargs), nil
}

func fnS3Cache(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return makeStruct("s3_cache", kwargs), nil
}

func fnSources(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return makeStruct("sources", kwargs), nil
}

func fnModule(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("module() requires a URL argument")
	}
	url, ok := args[0].(starlark.String)
	if !ok {
		return nil, fmt.Errorf("module() URL must be a string")
	}
	d := starlark.StringDict{"url": url}
	for _, kv := range kwargs {
		d[string(kv[0].(starlark.String))] = kv[1]
	}
	return starlarkstruct.FromStringDict(starlark.String("module"), d), nil
}

func fnKernel(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return makeStruct("kernel", kwargs), nil
}

func fnUboot(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return makeStruct("uboot", kwargs), nil
}

func fnQEMUConfig(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return makeStruct("qemu_config", kwargs), nil
}

func fnPartition(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return makeStruct("partition", kwargs), nil
}

// --- Built-in functions that register module info ---

func (e *Engine) fnModuleInfo(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := kwString(kwargs, "name")
	if name == "" {
		return nil, fmt.Errorf("module_info() requires name")
	}

	info := &ModuleInfo{
		Name:        name,
		Description: kwString(kwargs, "description"),
	}

	// Parse deps list of module() structs
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == "deps" {
			if list, ok := kv[1].(*starlark.List); ok {
				iter := list.Iterate()
				defer iter.Done()
				var v starlark.Value
				for iter.Next(&v) {
					if s, ok := v.(*starlarkstruct.Struct); ok {
						info.Deps = append(info.Deps, ModuleRef{
							URL: structString(s, "url"),
							Ref: structString(s, "ref"),
						})
					}
				}
			}
		}
	}

	e.moduleInfo = info
	return starlark.None, nil
}

// --- Built-in functions that register targets (side-effecting) ---

func (e *Engine) fnProject(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.project != nil {
		return nil, fmt.Errorf("project() called more than once")
	}

	defs := kwStruct(kwargs, "defaults")
	cacheS := kwStruct(kwargs, "cache")

	p := &Project{
		Name:    kwString(kwargs, "name"),
		Version: kwString(kwargs, "version"),
		Defaults: Defaults{
			Machine: structString(defs, "machine"),
			Image:   structString(defs, "image"),
		},
		Cache: CacheConfig{
			Path: structString(cacheS, "path"),
		},
	}

	// Parse modules list
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == "modules" {
			if list, ok := kv[1].(*starlark.List); ok {
				iter := list.Iterate()
				defer iter.Done()
				var v starlark.Value
				for iter.Next(&v) {
					if s, ok := v.(*starlarkstruct.Struct); ok {
						p.Modules = append(p.Modules, ModuleRef{
							URL:   structString(s, "url"),
							Ref:   structString(s, "ref"),
							Path:  structString(s, "path"),
							Local: structString(s, "local"),
						})
					}
				}
			}
		}
	}

	e.project = p
	return starlark.None, nil
}

func (e *Engine) fnMachine(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	name := kwString(kwargs, "name")
	arch := kwString(kwargs, "arch")

	if name == "" {
		return nil, fmt.Errorf("machine() requires name")
	}
	if !validArchitectures[arch] {
		return nil, fmt.Errorf("machine %q: invalid arch %q (valid: arm64, riscv64, x86_64)", name, arch)
	}

	kernelS := kwStruct(kwargs, "kernel")

	m := &Machine{
		Name:        name,
		Arch:        arch,
		Description: kwString(kwargs, "description"),
		Kernel: KernelConfig{
			Repo:        structString(kernelS, "repo"),
			Branch:      structString(kernelS, "branch"),
			Tag:         structString(kernelS, "tag"),
			Defconfig:   structString(kernelS, "defconfig"),
			DeviceTrees: structStringList(kernelS, "device_trees"),
			Unit:        structString(kernelS, "unit"),
			Cmdline:     structString(kernelS, "cmdline"),
			Provides:    structString(kernelS, "provides"),
		},
		Packages: kwStringList(kwargs, "packages"),
	}

	// Handle bootloader, qemu, and partitions from kwargs
	for _, kv := range kwargs {
		key := string(kv[0].(starlark.String))
		switch key {
		case "bootloader", "uboot", "qemu":
			s, ok := kv[1].(*starlarkstruct.Struct)
			if !ok {
				continue
			}
			switch key {
			case "bootloader":
				m.Bootloader = BootloaderConfig{
					Type:      structString(s, "type"),
					Repo:      structString(s, "repo"),
					Branch:    structString(s, "branch"),
					Defconfig: structString(s, "defconfig"),
				}
			case "uboot":
				m.Bootloader = BootloaderConfig{
					Type:      "u-boot",
					Repo:      structString(s, "repo"),
					Branch:    structString(s, "branch"),
					Defconfig: structString(s, "defconfig"),
				}
			case "qemu":
				m.QEMU = &QEMUConfig{
					Machine:  structString(s, "machine"),
					CPU:      structString(s, "cpu"),
					Memory:   structString(s, "memory"),
					Firmware: structString(s, "firmware"),
					Display:  structString(s, "display"),
				}
			}
		case "partitions":
			if list, ok := kv[1].(*starlark.List); ok {
				iter := list.Iterate()
				defer iter.Done()
				var v starlark.Value
				for iter.Next(&v) {
					if s, ok := v.(*starlarkstruct.Struct); ok {
						p := Partition{
							Label:    structString(s, "label"),
							Type:     structString(s, "type"),
							Size:     structString(s, "size"),
							Contents: structStringList(s, "contents"),
						}
						if rv, err := s.Attr("root"); err == nil {
							if b, ok := rv.(starlark.Bool); ok {
								p.Root = bool(b)
							}
						}
						m.Partitions = append(m.Partitions, p)
					}
				}
			}
		}
	}

	e.mu.Lock()
	e.machines[name] = m
	e.mu.Unlock()

	return starlark.None, nil
}

func (e *Engine) registerUnit(class string, kwargs []starlark.Tuple) (*Unit, error) {
	name := kwString(kwargs, "name")
	if name == "" {
		return nil, fmt.Errorf("%s() requires name", class)
	}

	// Allow Starlark to override class (e.g., image() class calls unit() with unit_class="image")
	cls := kwString(kwargs, "unit_class")
	if cls == "" {
		cls = class
	}

	r := &Unit{
		Name:        name,
		Version:     kwString(kwargs, "version"),
		Release:     kwInt(kwargs, "release"),
		Class:       cls,
		Scope:       kwString(kwargs, "scope"),
		Description: kwString(kwargs, "description"),
		License:     kwString(kwargs, "license"),
		Source:      kwString(kwargs, "source"),
		SHA256:      kwString(kwargs, "sha256"),
		Tag:         kwString(kwargs, "tag"),
		Branch:      kwString(kwargs, "branch"),
		Patches:     kwStringList(kwargs, "patches"),
		Deps:        kwStringList(kwargs, "deps"),
		RuntimeDeps: kwStringList(kwargs, "runtime_deps"),
		Container:     kwString(kwargs, "container"),
		ContainerArch: kwString(kwargs, "container_arch"),
		Provides:    kwString(kwargs, "provides"),
		Services:    kwStringList(kwargs, "services"),
		Conffiles:   kwStringList(kwargs, "conffiles"),
		Artifacts:   kwStringList(kwargs, "artifacts"),
		Exclude:     kwStringList(kwargs, "exclude"),
		Hostname:    kwString(kwargs, "hostname"),
		Timezone:    kwString(kwargs, "timezone"),
		Locale:      kwString(kwargs, "locale"),
	}

	// Parse tasks
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == "tasks" {
			if list, ok := kv[1].(*starlark.List); ok {
				iter := list.Iterate()
				defer iter.Done()
				var v starlark.Value
				for iter.Next(&v) {
					s, ok := v.(*starlarkstruct.Struct)
					if !ok {
						continue
					}
					t := Task{
						Name:      structString(s, "name"),
						Container: structString(s, "container"),
					}
					// Parse steps: run (single string), fn (single callable),
					// or steps (list of strings/callables)
					if rv, err := s.Attr("run"); err == nil {
						if cmd, ok := rv.(starlark.String); ok {
							t.Steps = []Step{{Command: string(cmd)}}
						}
					}
					if rv, err := s.Attr("fn"); err == nil {
						if fn, ok := rv.(starlark.Callable); ok {
							t.Steps = []Step{{Fn: fn}}
						}
					}
					if rv, err := s.Attr("steps"); err == nil {
						if list, ok := rv.(*starlark.List); ok {
							si := list.Iterate()
							var sv starlark.Value
							for si.Next(&sv) {
								switch val := sv.(type) {
								case starlark.String:
									t.Steps = append(t.Steps, Step{Command: string(val)})
								case starlark.Callable:
									t.Steps = append(t.Steps, Step{Fn: val})
								}
							}
							si.Done()
						}
					}
					r.Tasks = append(r.Tasks, t)
				}
			}
		}
	}

	// Parse partitions if present
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == "partitions" {
			if list, ok := kv[1].(*starlark.List); ok {
				iter := list.Iterate()
				defer iter.Done()
				var v starlark.Value
				for iter.Next(&v) {
					if s, ok := v.(*starlarkstruct.Struct); ok {
						p := Partition{
							Label:    structString(s, "label"),
							Type:     structString(s, "type"),
							Size:     structString(s, "size"),
							Contents: structStringList(s, "contents"),
						}
						if rv, err := s.Attr("root"); err == nil {
							if b, ok := rv.(starlark.Bool); ok {
								p.Root = bool(b)
							}
						}
						r.Partitions = append(r.Partitions, p)
					}
				}
			}
		}
	}

	r.Module = e.currentModule
	r.ModuleIndex = e.currentModuleIndex
	if e.currentFile != "" {
		r.DefinedIn = filepath.Dir(e.currentFile)
	}

	e.mu.Lock()
	if existing, ok := e.units[name]; ok {
		e.mu.Unlock()
		src := "project root"
		if existing.Module != "" {
			src = fmt.Sprintf("module %q", existing.Module)
		}
		return nil, fmt.Errorf("unit %q already defined (first defined in %s)", name, src)
	}
	e.units[name] = r
	e.mu.Unlock()

	return r, nil
}

func (e *Engine) fnUnit(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	_, err := e.registerUnit("unit", kwargs)
	return starlark.None, err
}

func (e *Engine) fnImage(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	_, err := e.registerUnit("image", kwargs)
	return starlark.None, err
}

// --- Task builtin ---

func fnTask(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name starlark.String
	if err := starlark.UnpackPositionalArgs("task", args, nil, 1, &name); err != nil {
		return nil, err
	}

	fields := starlark.StringDict{
		"name": name,
	}

	for _, kv := range kwargs {
		key := string(kv[0].(starlark.String))
		fields[key] = kv[1]
	}

	return starlarkstruct.FromStringDict(starlark.String("task"), fields), nil
}

// --- Custom commands ---

func fnArg(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("arg() requires a name")
	}
	name, ok := args[0].(starlark.String)
	if !ok {
		return nil, fmt.Errorf("arg() name must be a string")
	}
	d := starlark.StringDict{"name": name}
	for _, kv := range kwargs {
		d[string(kv[0].(starlark.String))] = kv[1]
	}
	return starlarkstruct.FromStringDict(starlark.String("arg"), d), nil
}

func (e *Engine) fnCommand(thread *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	name := kwString(kwargs, "name")
	if name == "" {
		return nil, fmt.Errorf("command() requires name")
	}

	cmd := &Command{
		Name:        name,
		Description: kwString(kwargs, "description"),
		SourceFile:  thread.Name,
	}

	// Parse args list
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == "args" {
			if list, ok := kv[1].(*starlark.List); ok {
				iter := list.Iterate()
				defer iter.Done()
				var v starlark.Value
				for iter.Next(&v) {
					if s, ok := v.(*starlarkstruct.Struct); ok {
						a := CommandArg{
							Name:    structString(s, "name"),
							Help:    structString(s, "help"),
							Default: structString(s, "default"),
						}
						if rv, err := s.Attr("required"); err == nil {
							if b, ok := rv.(starlark.Bool); ok {
								a.Required = bool(b)
							}
						}
						if rv, err := s.Attr("type"); err == nil {
							if str, ok := rv.(starlark.String); ok && string(str) == "bool" {
								a.IsBool = true
							}
						}
						cmd.Args = append(cmd.Args, a)
					}
				}
			}
		}
	}

	e.mu.Lock()
	e.commands[name] = cmd
	e.mu.Unlock()

	return starlark.None, nil
}

