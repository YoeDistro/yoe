package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
	"go.starlark.net/starlark"
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

// fnInstallFile implements install_file(src, dest, mode=0o644).
// Copies src verbatim from the unit's files directory to dest. Relative
// paths are resolved under <DefinedIn>/<unit-name>/. Environment variables
// like $DESTDIR are expanded in the destination path using the build env.
func fnInstallFile(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dest starlark.String
	if err := starlark.UnpackPositionalArgs("install_file", args, nil, 2, &src, &dest); err != nil {
		return nil, err
	}
	mode, err := modeFromKwargs("install_file", kwargs, 0644)
	if err != nil {
		return nil, err
	}

	tctx, ok := thread.Local(templateKey).(*TemplateContext)
	if !ok || tctx == nil {
		return nil, fmt.Errorf("install_file: no template context on thread (called outside a task fn?)")
	}

	srcPath, err := resolveTemplatePath(tctx.Unit, string(src))
	if err != nil {
		return nil, fmt.Errorf("install_file: %w", err)
	}
	destPath := expandEnv(string(dest), tctx.Env)

	content, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("install_file: reading %s: %w", srcPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("install_file: creating dir for %s: %w", destPath, err)
	}
	if err := os.WriteFile(destPath, content, os.FileMode(mode)); err != nil {
		return nil, fmt.Errorf("install_file: writing %s: %w", destPath, err)
	}
	return starlark.None, nil
}

// fnInstallTemplate implements install_template(src, dest, mode=0o644).
// Reads a Go text/template from the unit's files directory, renders it with
// the unit's context map, and writes the result to dest. Missing keys in the
// template cause an error rather than a silent empty substitution.
func fnInstallTemplate(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dest starlark.String
	if err := starlark.UnpackPositionalArgs("install_template", args, nil, 2, &src, &dest); err != nil {
		return nil, err
	}
	mode, err := modeFromKwargs("install_template", kwargs, 0644)
	if err != nil {
		return nil, err
	}

	tctx, ok := thread.Local(templateKey).(*TemplateContext)
	if !ok || tctx == nil {
		return nil, fmt.Errorf("install_template: no template context on thread (called outside a task fn?)")
	}

	srcPath, err := resolveTemplatePath(tctx.Unit, string(src))
	if err != nil {
		return nil, fmt.Errorf("install_template: %w", err)
	}
	destPath := expandEnv(string(dest), tctx.Env)

	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("install_template: reading %s: %w", srcPath, err)
	}

	tmpl, err := template.New(filepath.Base(srcPath)).
		Option("missingkey=error").
		Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("install_template: parsing %s: %w", srcPath, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, tctx.Data); err != nil {
		return nil, fmt.Errorf("install_template: rendering %s: %w", srcPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("install_template: creating dir for %s: %w", destPath, err)
	}
	if err := os.WriteFile(destPath, []byte(buf.String()), os.FileMode(mode)); err != nil {
		return nil, fmt.Errorf("install_template: writing %s: %w", destPath, err)
	}
	return starlark.None, nil
}

// resolveTemplatePath resolves a relative path against the unit's files
// directory: <DefinedIn>/<unit-name>/<relPath>. Rejects paths that escape
// the unit files directory (e.g. "../../etc/passwd").
func resolveTemplatePath(u *yoestar.Unit, relPath string) (string, error) {
	filesDir := filepath.Join(u.DefinedIn, u.Name)
	resolved := filepath.Join(filesDir, relPath)
	rel, err := filepath.Rel(filesDir, resolved)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", relPath, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes unit files directory", relPath)
	}
	return resolved, nil
}

// expandEnv expands $VAR and ${VAR} references using the provided build env.
// Unknown variables expand to the empty string — we deliberately do NOT fall
// back to the host process environment, because that would break
// reproducibility and content-addressed caching.
func expandEnv(s string, env map[string]string) string {
	return os.Expand(s, func(key string) string {
		return env[key]
	})
}

// modeFromKwargs extracts the `mode` kwarg from a builtin's kwargs list,
// returning def if not set. Errors if present but not an int.
func modeFromKwargs(name string, kwargs []starlark.Tuple, def int) (int, error) {
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) != "mode" {
			continue
		}
		n, ok := kv[1].(starlark.Int)
		if !ok {
			return 0, fmt.Errorf("%s: mode must be int, got %s", name, kv[1].Type())
		}
		v, ok := n.Int64()
		if !ok {
			return 0, fmt.Errorf("%s: mode out of range", name)
		}
		return int(v), nil
	}
	return def, nil
}
