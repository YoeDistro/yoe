package starlark

import (
	"path/filepath"
	"testing"
)

func TestLoadProject(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "valid-project")
	proj, err := LoadProject(dir)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	if proj.Name != "test-distro" {
		t.Errorf("Name = %q, want %q", proj.Name, "test-distro")
	}
	if proj.Defaults.Machine != "qemu-x86_64" {
		t.Errorf("Defaults.Machine = %q, want %q", proj.Defaults.Machine, "qemu-x86_64")
	}

	// Machines
	if len(proj.Machines) != 2 {
		t.Errorf("got %d machines, want 2", len(proj.Machines))
	}
	if m, ok := proj.Machines["beaglebone-black"]; !ok {
		t.Error("expected machine 'beaglebone-black'")
	} else if m.Arch != "arm64" {
		t.Errorf("bbb arch = %q, want %q", m.Arch, "arm64")
	}
	if m, ok := proj.Machines["qemu-x86_64"]; !ok {
		t.Error("expected machine 'qemu-x86_64'")
	} else if m.QEMU == nil {
		t.Error("expected QEMU config on qemu-x86_64")
	}

	// Recipes
	if len(proj.Recipes) != 5 {
		t.Errorf("got %d recipes, want 5", len(proj.Recipes))
	}
	if r, ok := proj.Recipes["openssh"]; !ok {
		t.Error("expected recipe 'openssh'")
	} else if r.Class != "package" {
		t.Errorf("openssh class = %q, want %q", r.Class, "package")
	}
	if r, ok := proj.Recipes["myapp"]; !ok {
		t.Error("expected recipe 'myapp'")
	} else if r.Class != "go" {
		t.Errorf("myapp class = %q, want %q", r.Class, "go")
	}
	if r, ok := proj.Recipes["base-image"]; !ok {
		t.Error("expected recipe 'base-image'")
	} else {
		if r.Class != "image" {
			t.Errorf("base-image class = %q, want %q", r.Class, "image")
		}
		if len(r.Partitions) != 2 {
			t.Errorf("base-image partitions = %d, want 2", len(r.Partitions))
		}
	}
}

func TestLoadMinimalProject(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "minimal-project")
	proj, err := LoadProject(dir)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if proj.Name != "minimal" {
		t.Errorf("Name = %q, want %q", proj.Name, "minimal")
	}
}

func TestLoadProject_NotFound(t *testing.T) {
	_, err := LoadProject("/tmp")
	if err == nil {
		t.Fatal("expected error when no PROJECT.star found, got nil")
	}
}
