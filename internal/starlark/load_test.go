package starlark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFunction(t *testing.T) {
	// Create temp project with a class file and a unit that loads it.
	tmp := t.TempDir()

	// classes/myclass.star defines a helper function
	classesDir := filepath.Join(tmp, "classes")
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(classesDir, "myclass.star"), []byte(`
def my_builder(name, version):
    unit(name = name, version = version)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// units/hello.star loads the class and calls it
	unitsDir := filepath.Join(tmp, "units")
	if err := os.MkdirAll(unitsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unitsDir, "hello.star"), []byte(`
load("//classes/myclass.star", "my_builder")
my_builder(name = "hello", version = "1.0")
`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := NewEngine()
	eng.SetProjectRoot(tmp)

	if err := eng.ExecFile(filepath.Join(unitsDir, "hello.star")); err != nil {
		t.Fatalf("ExecFile: %v", err)
	}

	r, ok := eng.Units()["hello"]
	if !ok {
		t.Fatal("unit 'hello' not registered")
	}
	if r.Class != "unit" {
		t.Errorf("Class = %q, want %q", r.Class, "unit")
	}
	if r.Version != "1.0" {
		t.Errorf("Version = %q, want %q", r.Version, "1.0")
	}
}

func TestLoadFunction_ModuleRef(t *testing.T) {
	tmp := t.TempDir()

	// Create a module directory with a helper class
	layerDir := filepath.Join(tmp, "modules", "mylib")
	classesDir := filepath.Join(layerDir, "classes")
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(classesDir, "helper.star"), []byte(`
def helper(name, version):
    unit(name = name, version = version)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a unit that loads from the module
	unitsDir := filepath.Join(tmp, "units")
	if err := os.MkdirAll(unitsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unitsDir, "widget.star"), []byte(`
load("@mylib//classes/helper.star", "helper")
helper(name = "widget", version = "2.0")
`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := NewEngine()
	eng.SetProjectRoot(tmp)
	eng.SetModuleRoot("mylib", layerDir)

	if err := eng.ExecFile(filepath.Join(unitsDir, "widget.star")); err != nil {
		t.Fatalf("ExecFile: %v", err)
	}

	r, ok := eng.Units()["widget"]
	if !ok {
		t.Fatal("unit 'widget' not registered")
	}
	if r.Class != "unit" {
		t.Errorf("Class = %q, want %q", r.Class, "unit")
	}
	if r.Version != "2.0" {
		t.Errorf("Version = %q, want %q", r.Version, "2.0")
	}
}

func TestLoadFunction_Cache(t *testing.T) {
	tmp := t.TempDir()

	// A class file
	classesDir := filepath.Join(tmp, "classes")
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(classesDir, "shared.star"), []byte(`
def shared_builder(name, version):
    unit(name = name, version = version)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Two units that load the same module
	unitsDir := filepath.Join(tmp, "units")
	if err := os.MkdirAll(unitsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unitsDir, "a.star"), []byte(`
load("//classes/shared.star", "shared_builder")
shared_builder(name = "pkg-a", version = "1.0")
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unitsDir, "b.star"), []byte(`
load("//classes/shared.star", "shared_builder")
shared_builder(name = "pkg-b", version = "2.0")
`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := NewEngine()
	eng.SetProjectRoot(tmp)

	if err := eng.ExecFile(filepath.Join(unitsDir, "a.star")); err != nil {
		t.Fatalf("ExecFile a.star: %v", err)
	}
	if err := eng.ExecFile(filepath.Join(unitsDir, "b.star")); err != nil {
		t.Fatalf("ExecFile b.star: %v", err)
	}

	if _, ok := eng.Units()["pkg-a"]; !ok {
		t.Error("unit 'pkg-a' not registered")
	}
	if _, ok := eng.Units()["pkg-b"]; !ok {
		t.Error("unit 'pkg-b' not registered")
	}

	// Verify cache was used (same module path should have one entry)
	absPath := filepath.Join(tmp, "classes", "shared.star")
	eng.loadCache.mu.Lock()
	entry, ok := eng.loadCache.entries[absPath]
	eng.loadCache.mu.Unlock()
	if !ok {
		t.Error("expected cache entry for shared.star")
	}
	if entry == nil || entry.err != nil {
		t.Error("expected successful cache entry")
	}
}
