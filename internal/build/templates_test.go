package build

import (
	"testing"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
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
