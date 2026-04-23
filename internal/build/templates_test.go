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
