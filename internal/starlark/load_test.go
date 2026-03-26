package starlark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFunction(t *testing.T) {
	// Create temp project with a class file and a recipe that loads it.
	tmp := t.TempDir()

	// classes/myclass.star defines a helper function
	classesDir := filepath.Join(tmp, "classes")
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(classesDir, "myclass.star"), []byte(`
def my_builder(name, version):
    autotools(name = name, version = version)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// recipes/hello.star loads the class and calls it
	recipesDir := filepath.Join(tmp, "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipesDir, "hello.star"), []byte(`
load("//classes/myclass.star", "my_builder")
my_builder(name = "hello", version = "1.0")
`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := NewEngine()
	eng.SetProjectRoot(tmp)

	if err := eng.ExecFile(filepath.Join(recipesDir, "hello.star")); err != nil {
		t.Fatalf("ExecFile: %v", err)
	}

	r, ok := eng.Recipes()["hello"]
	if !ok {
		t.Fatal("recipe 'hello' not registered")
	}
	if r.Class != "autotools" {
		t.Errorf("Class = %q, want %q", r.Class, "autotools")
	}
	if r.Version != "1.0" {
		t.Errorf("Version = %q, want %q", r.Version, "1.0")
	}
}

func TestLoadFunction_LayerRef(t *testing.T) {
	tmp := t.TempDir()

	// Create a layer directory with a helper class
	layerDir := filepath.Join(tmp, "layers", "mylib")
	classesDir := filepath.Join(layerDir, "classes")
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(classesDir, "helper.star"), []byte(`
def helper(name, version):
    autotools(name = name, version = version)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a recipe that loads from the layer
	recipesDir := filepath.Join(tmp, "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipesDir, "widget.star"), []byte(`
load("@mylib//classes/helper.star", "helper")
helper(name = "widget", version = "2.0")
`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := NewEngine()
	eng.SetProjectRoot(tmp)
	eng.SetLayerRoot("mylib", layerDir)

	if err := eng.ExecFile(filepath.Join(recipesDir, "widget.star")); err != nil {
		t.Fatalf("ExecFile: %v", err)
	}

	r, ok := eng.Recipes()["widget"]
	if !ok {
		t.Fatal("recipe 'widget' not registered")
	}
	if r.Class != "autotools" {
		t.Errorf("Class = %q, want %q", r.Class, "autotools")
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
    autotools(name = name, version = version)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Two recipes that load the same module
	recipesDir := filepath.Join(tmp, "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipesDir, "a.star"), []byte(`
load("//classes/shared.star", "shared_builder")
shared_builder(name = "pkg-a", version = "1.0")
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipesDir, "b.star"), []byte(`
load("//classes/shared.star", "shared_builder")
shared_builder(name = "pkg-b", version = "2.0")
`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := NewEngine()
	eng.SetProjectRoot(tmp)

	if err := eng.ExecFile(filepath.Join(recipesDir, "a.star")); err != nil {
		t.Fatalf("ExecFile a.star: %v", err)
	}
	if err := eng.ExecFile(filepath.Join(recipesDir, "b.star")); err != nil {
		t.Fatalf("ExecFile b.star: %v", err)
	}

	if _, ok := eng.Recipes()["pkg-a"]; !ok {
		t.Error("recipe 'pkg-a' not registered")
	}
	if _, ok := eng.Recipes()["pkg-b"]; !ok {
		t.Error("recipe 'pkg-b' not registered")
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
