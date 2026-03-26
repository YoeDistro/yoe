package config

import (
	"path/filepath"
	"testing"
)

func TestLoadProject(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "valid-project")
	project, err := LoadProject(dir)
	if err != nil {
		t.Fatalf("LoadProject(%q): %v", dir, err)
	}

	if project.Distro.Distro.Name != "test-distro" {
		t.Errorf("Distro.Name = %q, want %q", project.Distro.Distro.Name, "test-distro")
	}
	if len(project.Machines) == 0 {
		t.Error("expected at least one machine, got 0")
	}
	if _, ok := project.Machines["beaglebone-black"]; !ok {
		t.Error("expected machine 'beaglebone-black' to be loaded")
	}
	if len(project.Recipes) == 0 {
		t.Error("expected at least one recipe, got 0")
	}
	if _, ok := project.Recipes["openssh"]; !ok {
		t.Error("expected recipe 'openssh' to be loaded")
	}
	if len(project.Images) == 0 {
		t.Error("expected at least one image, got 0")
	}
	if _, ok := project.Images["base"]; !ok {
		t.Error("expected image 'base' to be loaded")
	}
}
