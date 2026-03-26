package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func setupTestProject(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "test-project")
	if err := RunInit(dir, "qemu-x86_64"); err != nil {
		t.Fatalf("init: %v", err)
	}
	return dir
}

func TestConfigShow(t *testing.T) {
	dir := setupTestProject(t)

	var buf bytes.Buffer
	if err := RunConfigShow(dir, &buf); err != nil {
		t.Fatalf("RunConfigShow: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("config show produced no output")
	}
}

func TestConfigSet(t *testing.T) {
	dir := setupTestProject(t)

	if err := RunConfigSet(dir, "defaults.machine", "beaglebone-black"); err != nil {
		t.Fatalf("RunConfigSet: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "distro.toml"))
	if err != nil {
		t.Fatalf("reading distro.toml: %v", err)
	}
	if !bytes.Contains(data, []byte("beaglebone-black")) {
		t.Error("distro.toml does not contain updated machine name")
	}
}
