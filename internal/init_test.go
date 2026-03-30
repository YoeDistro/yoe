package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunInit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-project")

	if err := RunInit(dir, ""); err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	for _, path := range []string{
		"PROJECT.star",
		".gitignore",
		"machines",
		"units",
		"classes",
		"overlays",
	} {
		full := filepath.Join(dir, path)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after init", path)
		}
	}

	// Verify PROJECT.star is valid Starlark
	content, err := os.ReadFile(filepath.Join(dir, "PROJECT.star"))
	if err != nil {
		t.Fatalf("reading PROJECT.star: %v", err)
	}
	if len(content) == 0 {
		t.Error("PROJECT.star is empty")
	}
}

func TestRunInit_WithMachine(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-project")

	if err := RunInit(dir, "qemu-x86_64"); err != nil {
		t.Fatalf("RunInit with machine: %v", err)
	}

	machineFile := filepath.Join(dir, "machines", "qemu-x86_64.star")
	if _, err := os.Stat(machineFile); os.IsNotExist(err) {
		t.Errorf("expected machine file %s to exist", machineFile)
	}
}

func TestRunInit_ExistingProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "PROJECT.star"), []byte("project(name=\"exists\")\n"), 0644)

	if err := RunInit(dir, ""); err == nil {
		t.Fatal("expected error when init into existing project, got nil")
	}
}
