package build

import (
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// templateKey is the thread-local key under which the build executor stores
// the per-unit TemplateContext for install_file / install_template to read.
const templateKey = "yoe.template"

// TemplateContext carries the per-unit state needed to resolve template
// paths, render templates, and write output files during a task's fn step.
type TemplateContext struct {
	Unit *yoestar.Unit     // for DefinedIn and Name (path resolution)
	Data map[string]any    // rendered as Go template data
	Env  map[string]string // DESTDIR, PREFIX, etc. for destination path expansion
}

// BuildTemplateContext builds the context map passed to Go templates, merging
// auto-populated fields (arch, machine, console, project) and unit identity
// fields (name, version, release) with the unit's Extra kwargs. Extra wins
// on key collision so explicit unit fields always override defaults.
func BuildTemplateContext(u *yoestar.Unit, arch, machine, console, project string) map[string]any {
	m := map[string]any{
		"name":    u.Name,
		"version": u.Version,
		"release": int64(u.Release),
		"arch":    arch,
		"machine": machine,
		"console": console,
		"project": project,
	}
	for k, v := range u.Extra {
		m[k] = v
	}
	return m
}
