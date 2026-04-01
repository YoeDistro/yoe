package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// builtins returns the predeclared names available in all .star files.
func (e *Engine) builtins() starlark.StringDict {
	d := starlark.StringDict{
		"project":     starlark.NewBuiltin("project", e.fnProject),
		"defaults":    starlark.NewBuiltin("defaults", fnDefaults),
		"repository":  starlark.NewBuiltin("repository", fnRepository),
		"cache":       starlark.NewBuiltin("cache", fnCache),
		"s3_cache":    starlark.NewBuiltin("s3_cache", fnS3Cache),
		"sources":     starlark.NewBuiltin("sources", fnSources),
		"layer":       starlark.NewBuiltin("layer", fnLayer),
		"layer_info":  starlark.NewBuiltin("layer_info", e.fnLayerInfo),
		"machine":     starlark.NewBuiltin("machine", e.fnMachine),
		"kernel":      starlark.NewBuiltin("kernel", fnKernel),
		"uboot":       starlark.NewBuiltin("uboot", fnUboot),
		"qemu_config": starlark.NewBuiltin("qemu_config", fnQEMUConfig),
		"unit":        starlark.NewBuiltin("unit", e.fnUnit),
		"autotools":   starlark.NewBuiltin("autotools", e.fnAutotools),
		"cmake":       starlark.NewBuiltin("cmake", e.fnCMake),
		"go_binary":   starlark.NewBuiltin("go_binary", e.fnGoBinary),
		"image":       starlark.NewBuiltin("image", e.fnImage),
		"partition":   starlark.NewBuiltin("partition", fnPartition),
		"command":     starlark.NewBuiltin("command", e.fnCommand),
		"arg":         starlark.NewBuiltin("arg", fnArg),
		"True":        starlark.True,
		"False":       starlark.False,
	}

	// Merge engine variables (e.g., ARCH set after machine loading).
	for k, v := range e.vars {
		d[k] = v
	}

	return d
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

func fnRepository(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return makeStruct("repository", kwargs), nil
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

func fnLayer(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("layer() requires a URL argument")
	}
	url, ok := args[0].(starlark.String)
	if !ok {
		return nil, fmt.Errorf("layer() URL must be a string")
	}
	d := starlark.StringDict{"url": url}
	for _, kv := range kwargs {
		d[string(kv[0].(starlark.String))] = kv[1]
	}
	return starlarkstruct.FromStringDict(starlark.String("layer"), d), nil
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

// --- Built-in functions that register layer info ---

func (e *Engine) fnLayerInfo(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := kwString(kwargs, "name")
	if name == "" {
		return nil, fmt.Errorf("layer_info() requires name")
	}

	info := &LayerInfo{
		Name:        name,
		Description: kwString(kwargs, "description"),
	}

	// Parse deps list of layer() structs
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == "deps" {
			if list, ok := kv[1].(*starlark.List); ok {
				iter := list.Iterate()
				defer iter.Done()
				var v starlark.Value
				for iter.Next(&v) {
					if s, ok := v.(*starlarkstruct.Struct); ok {
						info.Deps = append(info.Deps, LayerRef{
							URL: structString(s, "url"),
							Ref: structString(s, "ref"),
						})
					}
				}
			}
		}
	}

	e.layerInfo = info
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
	repo := kwStruct(kwargs, "repository")
	cacheS := kwStruct(kwargs, "cache")

	p := &Project{
		Name:    kwString(kwargs, "name"),
		Version: kwString(kwargs, "version"),
		Defaults: Defaults{
			Machine: structString(defs, "machine"),
			Image:   structString(defs, "image"),
		},
		Repository: RepositoryConfig{
			Path: structString(repo, "path"),
		},
		Cache: CacheConfig{
			Path: structString(cacheS, "path"),
		},
	}

	// Parse layers list
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == "layers" {
			if list, ok := kv[1].(*starlark.List); ok {
				iter := list.Iterate()
				defer iter.Done()
				var v starlark.Value
				for iter.Next(&v) {
					if s, ok := v.(*starlarkstruct.Struct); ok {
						p.Layers = append(p.Layers, LayerRef{
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
		},
	}

	// Handle bootloader and qemu from kwargs
	for _, kv := range kwargs {
		key := string(kv[0].(starlark.String))
		if s, ok := kv[1].(*starlarkstruct.Struct); ok {
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

	r := &Unit{
		Name:          name,
		Version:       kwString(kwargs, "version"),
		Class:         class,
		Scope:         kwString(kwargs, "scope"),
		Description:   kwString(kwargs, "description"),
		License:       kwString(kwargs, "license"),
		Source:        kwString(kwargs, "source"),
		SHA256:        kwString(kwargs, "sha256"),
		Tag:           kwString(kwargs, "tag"),
		Branch:        kwString(kwargs, "branch"),
		Patches:       kwStringList(kwargs, "patches"),
		Deps:          kwStringList(kwargs, "deps"),
		RuntimeDeps:   kwStringList(kwargs, "runtime_deps"),
		Build:         kwStringList(kwargs, "build"),
		ConfigureArgs: kwStringList(kwargs, "configure_args"),
		GoPackage:     kwString(kwargs, "package"),
		Services:      kwStringList(kwargs, "services"),
		Conffiles:     kwStringList(kwargs, "conffiles"),
		Artifacts:     kwStringList(kwargs, "artifacts"),
		Exclude:       kwStringList(kwargs, "exclude"),
		Hostname:      kwString(kwargs, "hostname"),
		Timezone:      kwString(kwargs, "timezone"),
		Locale:        kwString(kwargs, "locale"),
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

	e.mu.Lock()
	e.units[name] = r
	e.mu.Unlock()

	return r, nil
}

func (e *Engine) fnUnit(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	r, err := e.registerUnit("unit", kwargs)
	if err != nil {
		return nil, err
	}
	if len(r.Build) == 0 {
		return nil, fmt.Errorf("unit(%q): build steps required", r.Name)
	}
	return starlark.None, nil
}

func (e *Engine) fnAutotools(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	_, err := e.registerUnit("autotools", kwargs)
	return starlark.None, err
}

func (e *Engine) fnCMake(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	_, err := e.registerUnit("cmake", kwargs)
	return starlark.None, err
}

func (e *Engine) fnGoBinary(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	_, err := e.registerUnit("go", kwargs)
	return starlark.None, err
}

func (e *Engine) fnImage(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	_, err := e.registerUnit("image", kwargs)
	return starlark.None, err
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

