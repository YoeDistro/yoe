package config

import (
	"path/filepath"
	"testing"
)

func TestFindProjectRoot(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "valid-project")
	root, err := FindProjectRoot(dir)
	if err != nil {
		t.Fatalf("FindProjectRoot(%q): %v", dir, err)
	}
	absDir, _ := filepath.Abs(dir)
	if root != absDir {
		t.Errorf("FindProjectRoot = %q, want %q", root, absDir)
	}
}

func TestFindProjectRoot_NotFound(t *testing.T) {
	_, err := FindProjectRoot("/tmp")
	if err == nil {
		t.Fatal("expected error when no distro.toml found, got nil")
	}
}
