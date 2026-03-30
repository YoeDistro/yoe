package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunClean_Default(t *testing.T) {
	proj := t.TempDir()
	buildDir := filepath.Join(proj, "build")
	repoDir := filepath.Join(proj, "repo")

	// Create build and repo dirs with some content.
	for _, d := range []string{
		filepath.Join(buildDir, "foo"),
		filepath.Join(repoDir, "bar"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Default clean removes build but preserves repo.
	if err := RunClean(proj, false, true, nil); err != nil {
		t.Fatalf("RunClean default: %v", err)
	}

	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		t.Error("expected build dir to be removed")
	}
	if _, err := os.Stat(repoDir); err != nil {
		t.Error("expected repo dir to still exist")
	}
}

func TestRunClean_All(t *testing.T) {
	proj := t.TempDir()
	buildDir := filepath.Join(proj, "build")
	repoDir := filepath.Join(proj, "repo")

	for _, d := range []string{buildDir, repoDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := RunClean(proj, true, true, nil); err != nil {
		t.Fatalf("RunClean all: %v", err)
	}

	for _, d := range []string{buildDir, repoDir} {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed", d)
		}
	}
}

func TestRunClean_Units(t *testing.T) {
	proj := t.TempDir()
	buildDir := filepath.Join(proj, "build")

	// Create build dirs for two units.
	for _, r := range []string{"openssl", "busybox"} {
		if err := os.MkdirAll(filepath.Join(buildDir, r), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Clean only openssl.
	if err := RunClean(proj, false, true, []string{"openssl"}); err != nil {
		t.Fatalf("RunClean units: %v", err)
	}

	if _, err := os.Stat(filepath.Join(buildDir, "openssl")); !os.IsNotExist(err) {
		t.Error("expected openssl build dir to be removed")
	}
	if _, err := os.Stat(filepath.Join(buildDir, "busybox")); err != nil {
		t.Error("expected busybox build dir to still exist")
	}
}

func TestRunClean_NoBuildDir(t *testing.T) {
	proj := t.TempDir()

	// Should succeed even when build dir does not exist.
	if err := RunClean(proj, false, true, nil); err != nil {
		t.Fatalf("RunClean on missing build dir: %v", err)
	}
}
