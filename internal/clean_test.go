package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunClean(t *testing.T) {
	dir := setupTestProject(t)

	buildDir := filepath.Join(dir, "build")
	os.MkdirAll(buildDir, 0755)
	os.WriteFile(filepath.Join(buildDir, "artifact.o"), []byte("fake"), 0644)

	if err := RunClean(dir, false); err != nil {
		t.Fatalf("RunClean: %v", err)
	}

	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		t.Error("expected build directory to be removed")
	}
}

func TestRunClean_All(t *testing.T) {
	dir := setupTestProject(t)

	for _, sub := range []string{"build", "packages", "sources"} {
		d := filepath.Join(dir, sub)
		os.MkdirAll(d, 0755)
		os.WriteFile(filepath.Join(d, "artifact"), []byte("fake"), 0644)
	}

	if err := RunClean(dir, true); err != nil {
		t.Fatalf("RunClean --all: %v", err)
	}

	for _, sub := range []string{"build", "packages", "sources"} {
		d := filepath.Join(dir, sub)
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("expected %s directory to be removed", sub)
		}
	}
}
