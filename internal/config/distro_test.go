package config

import (
	"path/filepath"
	"testing"
)

func TestParseDistroConfig(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-project", "distro.toml")
	distro, err := ParseDistroConfig(path)
	if err != nil {
		t.Fatalf("ParseDistroConfig(%q): %v", path, err)
	}

	if distro.Distro.Name != "test-distro" {
		t.Errorf("Name = %q, want %q", distro.Distro.Name, "test-distro")
	}
	if distro.Distro.Version != "0.1.0" {
		t.Errorf("Version = %q, want %q", distro.Distro.Version, "0.1.0")
	}
	if distro.Defaults.Machine != "qemu-arm64" {
		t.Errorf("Defaults.Machine = %q, want %q", distro.Defaults.Machine, "qemu-arm64")
	}
	if distro.Defaults.Image != "base" {
		t.Errorf("Defaults.Image = %q, want %q", distro.Defaults.Image, "base")
	}
	if distro.Repository.Path != "/var/cache/yoe-ng/repo" {
		t.Errorf("Repository.Path = %q, want %q", distro.Repository.Path, "/var/cache/yoe-ng/repo")
	}
	if distro.Cache.Path != "/var/cache/yoe-ng/build" {
		t.Errorf("Cache.Path = %q, want %q", distro.Cache.Path, "/var/cache/yoe-ng/build")
	}
	if distro.Sources.GoProxy != "https://proxy.golang.org" {
		t.Errorf("Sources.GoProxy = %q, want %q", distro.Sources.GoProxy, "https://proxy.golang.org")
	}
}

func TestParseDistroConfig_MissingFile(t *testing.T) {
	_, err := ParseDistroConfig("nonexistent.toml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseDistroConfig_RequiredFields(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "invalid-project", "empty-distro.toml")
	_, err := ParseDistroConfig(path)
	if err == nil {
		t.Fatal("expected error for empty distro name, got nil")
	}
}
