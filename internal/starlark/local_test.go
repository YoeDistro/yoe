package starlark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLocalOverridesAbsentFile(t *testing.T) {
	dir := t.TempDir()
	ov, err := LoadLocalOverrides(dir)
	if err != nil {
		t.Fatalf("unexpected error for absent file: %v", err)
	}
	if ov.Machine != "" {
		t.Errorf("expected empty Machine, got %q", ov.Machine)
	}
}

func TestLoadLocalOverridesParsesMachine(t *testing.T) {
	dir := t.TempDir()
	content := `local(machine = "raspberrypi4")` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "local.star"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ov, err := LoadLocalOverrides(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ov.Machine != "raspberrypi4" {
		t.Errorf("Machine = %q, want raspberrypi4", ov.Machine)
	}
}

func TestLoadLocalOverridesUnknownKwarg(t *testing.T) {
	dir := t.TempDir()
	content := `local(arch = "arm64")` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "local.star"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLocalOverrides(dir); err == nil {
		t.Errorf("expected error for unknown kwarg, got nil")
	}
}

func TestWriteLocalOverridesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := WriteLocalOverrides(dir, LocalOverrides{Machine: "qemu-arm64"}); err != nil {
		t.Fatal(err)
	}
	ov, err := LoadLocalOverrides(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ov.Machine != "qemu-arm64" {
		t.Errorf("round-trip Machine = %q, want qemu-arm64", ov.Machine)
	}
}
