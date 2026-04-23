package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
	"go.starlark.net/starlark"
)

func TestBuildTemplateContext_MergesFields(t *testing.T) {
	u := &yoestar.Unit{
		Name:    "base-files",
		Version: "1.0.0",
		Release: 2,
		Extra: map[string]any{
			"port":      int64(8080),
			"log_level": "info",
		},
	}
	ctx := BuildTemplateContext(u, "x86_64", "qemu-x86_64", "ttyS0", "e2e-project")

	want := map[string]any{
		"name":      "base-files",
		"version":   "1.0.0",
		"release":   int64(2),
		"arch":      "x86_64",
		"machine":   "qemu-x86_64",
		"console":   "ttyS0",
		"project":   "e2e-project",
		"port":      int64(8080),
		"log_level": "info",
	}
	if len(ctx) != len(want) {
		t.Errorf("len(ctx) = %d, want %d; ctx = %v", len(ctx), len(want), ctx)
	}
	for k, v := range want {
		if ctx[k] != v {
			t.Errorf("ctx[%q] = %v (%T), want %v (%T)", k, ctx[k], ctx[k], v, v)
		}
	}
}

func TestBuildTemplateContext_ExtraOverridesAuto(t *testing.T) {
	u := &yoestar.Unit{
		Name:    "my-app",
		Version: "1.0.0",
		Extra: map[string]any{
			"machine": "override", // should win over auto-populated "qemu-x86_64"
		},
	}
	ctx := BuildTemplateContext(u, "x86_64", "qemu-x86_64", "ttyS0", "e2e-project")
	if ctx["machine"] != "override" {
		t.Errorf("ctx[machine] = %v, want \"override\" (Extra should override auto)", ctx["machine"])
	}
}

func TestFnInstallFile_CopiesVerbatim(t *testing.T) {
	tmp := t.TempDir()
	unitDir := filepath.Join(tmp, "unit-src", "my-unit")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := []byte("#!/bin/sh\necho hello\n")
	if err := os.WriteFile(filepath.Join(unitDir, "script.sh"), content, 0644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(tmp, "destdir")
	cfg := &SandboxConfig{
		Arch: "x86_64",
		Env:  map[string]string{"DESTDIR": destDir},
	}
	thread := NewBuildThread(context.Background(), cfg, RealExecer{})
	SetTemplateContext(thread, &TemplateContext{
		Unit: &yoestar.Unit{Name: "my-unit", DefinedIn: filepath.Join(tmp, "unit-src")},
		Data: map[string]any{},
		Env:  cfg.Env,
	})

	predeclared := BuildPredeclared()
	if _, err := starlark.ExecFile(thread, "test.star", `
install_file("script.sh", "$DESTDIR/usr/bin/script.sh", mode = 0o755)
`, predeclared); err != nil {
		t.Fatalf("ExecFile: %v", err)
	}

	destPath := filepath.Join(destDir, "usr/bin/script.sh")
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode = %o, want 0755", info.Mode().Perm())
	}
}

func TestFnInstallFile_MissingSrcErrors(t *testing.T) {
	tmp := t.TempDir()
	unitDir := filepath.Join(tmp, "unit-src", "my-unit")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatal(err)
	}
	destDir := filepath.Join(tmp, "destdir")
	cfg := &SandboxConfig{Arch: "x86_64", Env: map[string]string{"DESTDIR": destDir}}
	thread := NewBuildThread(context.Background(), cfg, RealExecer{})
	SetTemplateContext(thread, &TemplateContext{
		Unit: &yoestar.Unit{Name: "my-unit", DefinedIn: filepath.Join(tmp, "unit-src")},
		Data: map[string]any{},
		Env:  cfg.Env,
	})
	_, err := starlark.ExecFile(thread, "test.star",
		`install_file("does-not-exist.sh", "$DESTDIR/x")`, BuildPredeclared())
	if err == nil {
		t.Fatal("expected error when src missing, got nil")
	}
}

func TestFnInstallFile_NonIntModeErrors(t *testing.T) {
	tmp := t.TempDir()
	unitDir := filepath.Join(tmp, "unit-src", "u")
	_ = os.MkdirAll(unitDir, 0755)
	_ = os.WriteFile(filepath.Join(unitDir, "x"), []byte("x"), 0644)
	cfg := &SandboxConfig{Env: map[string]string{"DESTDIR": filepath.Join(tmp, "dd")}}
	thread := NewBuildThread(context.Background(), cfg, RealExecer{})
	SetTemplateContext(thread, &TemplateContext{
		Unit: &yoestar.Unit{Name: "u", DefinedIn: filepath.Join(tmp, "unit-src")},
		Data: map[string]any{},
		Env:  cfg.Env,
	})
	_, err := starlark.ExecFile(thread, "test.star",
		`install_file("x", "$DESTDIR/x", mode = "0755")`, BuildPredeclared())
	if err == nil {
		t.Fatal("expected error for non-int mode, got nil")
	}
}

func TestFnInstallFile_PathTraversalRejected(t *testing.T) {
	tmp := t.TempDir()
	unitDir := filepath.Join(tmp, "unit-src", "u")
	_ = os.MkdirAll(unitDir, 0755)
	cfg := &SandboxConfig{Env: map[string]string{"DESTDIR": filepath.Join(tmp, "dd")}}
	thread := NewBuildThread(context.Background(), cfg, RealExecer{})
	SetTemplateContext(thread, &TemplateContext{
		Unit: &yoestar.Unit{Name: "u", DefinedIn: filepath.Join(tmp, "unit-src")},
		Data: map[string]any{},
		Env:  cfg.Env,
	})
	_, err := starlark.ExecFile(thread, "test.star",
		`install_file("../../../etc/passwd", "$DESTDIR/x")`, BuildPredeclared())
	if err == nil {
		t.Fatal("expected path traversal rejection, got nil")
	}
}

func TestFnInstallTemplate_RendersWithContext(t *testing.T) {
	tmp := t.TempDir()
	unitDir := filepath.Join(tmp, "unit-src", "base-files")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatal(err)
	}
	tmplContent := `Machine: {{.machine}}
Console: {{.console}}
Version: {{.version}}
`
	if err := os.WriteFile(filepath.Join(unitDir, "info.tmpl"), []byte(tmplContent), 0644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(tmp, "destdir")
	cfg := &SandboxConfig{
		Arch: "x86_64",
		Env:  map[string]string{"DESTDIR": destDir},
	}
	thread := NewBuildThread(context.Background(), cfg, RealExecer{})
	SetTemplateContext(thread, &TemplateContext{
		Unit: &yoestar.Unit{
			Name:      "base-files",
			Version:   "1.0.0",
			DefinedIn: filepath.Join(tmp, "unit-src"),
		},
		Data: map[string]any{
			"name":    "base-files",
			"version": "1.0.0",
			"machine": "qemu-x86_64",
			"console": "ttyS0",
		},
		Env: cfg.Env,
	})

	predeclared := BuildPredeclared()
	if _, err := starlark.ExecFile(thread, "test.star", `
install_template("info.tmpl", "$DESTDIR/etc/info")
`, predeclared); err != nil {
		t.Fatalf("ExecFile: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "etc/info"))
	if err != nil {
		t.Fatal(err)
	}
	want := `Machine: qemu-x86_64
Console: ttyS0
Version: 1.0.0
`
	if string(got) != want {
		t.Errorf("rendered:\n%s\nwant:\n%s", got, want)
	}
}

func TestFnInstallTemplate_MissingKeyIsError(t *testing.T) {
	tmp := t.TempDir()
	unitDir := filepath.Join(tmp, "unit-src", "u")
	_ = os.MkdirAll(unitDir, 0755)
	_ = os.WriteFile(filepath.Join(unitDir, "x.tmpl"), []byte(`{{.missing}}`), 0644)

	cfg := &SandboxConfig{Env: map[string]string{"DESTDIR": filepath.Join(tmp, "dd")}}
	thread := NewBuildThread(context.Background(), cfg, RealExecer{})
	SetTemplateContext(thread, &TemplateContext{
		Unit: &yoestar.Unit{Name: "u", DefinedIn: filepath.Join(tmp, "unit-src")},
		Data: map[string]any{},
		Env:  cfg.Env,
	})

	predeclared := BuildPredeclared()
	_, err := starlark.ExecFile(thread, "test.star", `install_template("x.tmpl", "$DESTDIR/out")`, predeclared)
	if err == nil {
		t.Fatal("expected error on missing template key, got nil")
	}
}

func TestFnInstallTemplate_PathTraversalRejected(t *testing.T) {
	tmp := t.TempDir()
	unitDir := filepath.Join(tmp, "unit-src", "u")
	_ = os.MkdirAll(unitDir, 0755)
	cfg := &SandboxConfig{Env: map[string]string{"DESTDIR": filepath.Join(tmp, "dd")}}
	thread := NewBuildThread(context.Background(), cfg, RealExecer{})
	SetTemplateContext(thread, &TemplateContext{
		Unit: &yoestar.Unit{Name: "u", DefinedIn: filepath.Join(tmp, "unit-src")},
		Data: map[string]any{},
		Env:  cfg.Env,
	})

	predeclared := BuildPredeclared()
	_, err := starlark.ExecFile(thread, "test.star",
		`install_template("../../../etc/passwd", "$DESTDIR/x")`, predeclared)
	if err == nil {
		t.Fatal("expected path traversal rejection, got nil")
	}
}
