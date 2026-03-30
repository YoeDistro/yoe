package build

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

func TestE2E_DryRun(t *testing.T) {
	projectDir := filepath.Join("..", "..", "testdata", "e2e-project")
	if _, err := os.Stat(filepath.Join(projectDir, "PROJECT.star")); os.IsNotExist(err) {
		t.Skip("e2e test project not found")
	}

	proj, err := yoestar.LoadProject(projectDir)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	// Should have machine from units-core layer
	if _, ok := proj.Machines["qemu-x86_64"]; !ok {
		t.Error("expected qemu-x86_64 machine from units-core layer")
	}

	// Should have units from units-core layer
	if _, ok := proj.Units["busybox"]; !ok {
		t.Error("expected busybox unit from units-core layer")
	}
	if _, ok := proj.Units["linux"]; !ok {
		t.Error("expected linux unit from units-core layer")
	}
	if _, ok := proj.Units["base-image"]; !ok {
		t.Error("expected base-image from units-core layer")
	}
	if _, ok := proj.Units["zlib"]; !ok {
		t.Error("expected zlib unit from units-core layer")
	}

	// zlib should have been loaded via the autotools class
	if r := proj.Units["zlib"]; r != nil && r.Class != "unit" {
		// autotools() uses registerUnit, so class varies
		// but build steps should include ./configure
		if len(r.Build) < 3 {
			t.Errorf("zlib should have 3 build steps (autotools), got %d", len(r.Build))
		}
	}

	// Dry run should work
	var buf bytes.Buffer
	abs, _ := filepath.Abs(projectDir)
	opts := Options{
		DryRun:     true,
		ProjectDir: abs,
		Arch:       "x86_64",
	}

	if err := BuildUnits(proj, nil, opts, &buf); err != nil {
		t.Fatalf("BuildUnits dry run: %v", err)
	}

	output := buf.String()
	t.Logf("Dry run output:\n%s", output)

	if len(output) == 0 {
		t.Error("dry run produced no output")
	}
}
