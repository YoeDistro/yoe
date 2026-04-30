package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// runDirSizeMB evaluates `result = dir_size_mb(<arg>)` against a build
// thread whose DestDir is the given host path and returns the resulting
// integer.
func runDirSizeMB(t *testing.T, destDir, arg string) (int64, error) {
	t.Helper()
	cfg := &SandboxConfig{DestDir: destDir}
	thread := NewBuildThread(context.Background(), cfg, &fakeExecer{})
	predeclared := BuildPredeclared()
	src := "result = dir_size_mb(" + arg + ")\n"
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, "test.star", src, predeclared)
	if err != nil {
		return 0, err
	}
	r, ok := globals["result"].(starlark.Int)
	if !ok {
		t.Fatalf("result is not an int: %v", globals["result"])
	}
	v, ok := r.Int64()
	if !ok {
		t.Fatalf("result %v out of int64 range", r)
	}
	return v, nil
}

func TestDirSizeMB_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "rootfs"), 0755); err != nil {
		t.Fatal(err)
	}
	mb, err := runDirSizeMB(t, tmp, `"rootfs"`)
	if err != nil {
		t.Fatalf("dir_size_mb: %v", err)
	}
	if mb != 0 {
		t.Errorf("empty dir: got %d MB, want 0", mb)
	}
}

func TestDirSizeMB_SumsRegularFiles(t *testing.T) {
	tmp := t.TempDir()
	rootfs := filepath.Join(tmp, "rootfs")
	if err := os.MkdirAll(filepath.Join(rootfs, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	// 1 MiB + 1 byte file → rounds up to 2 MiB
	if err := os.WriteFile(filepath.Join(rootfs, "big"), make([]byte, 1024*1024+1), 0644); err != nil {
		t.Fatal(err)
	}
	// 100 KiB + 50 KiB across files → still under 1 MiB total but rounds up to 1 MiB
	if err := os.WriteFile(filepath.Join(rootfs, "sub", "a"), make([]byte, 100*1024), 0644); err != nil {
		t.Fatal(err)
	}
	mb, err := runDirSizeMB(t, tmp, `"rootfs"`)
	if err != nil {
		t.Fatalf("dir_size_mb: %v", err)
	}
	// big = 1 MiB + 1 byte, a = 100 KiB → total ~ 1.1 MiB → rounds up to 2
	if mb != 2 {
		t.Errorf("got %d MB, want 2", mb)
	}
}

func TestDirSizeMB_MissingDirReturnsZero(t *testing.T) {
	tmp := t.TempDir()
	mb, err := runDirSizeMB(t, tmp, `"does-not-exist"`)
	if err != nil {
		t.Fatalf("dir_size_mb: %v", err)
	}
	if mb != 0 {
		t.Errorf("missing dir: got %d MB, want 0", mb)
	}
}

func TestDirSizeMB_RejectsAbsolutePath(t *testing.T) {
	tmp := t.TempDir()
	_, err := runDirSizeMB(t, tmp, `"/etc"`)
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestDirSizeMB_RejectsParentTraversal(t *testing.T) {
	tmp := t.TempDir()
	_, err := runDirSizeMB(t, tmp, `"../etc"`)
	if err == nil {
		t.Fatal("expected error for ..")
	}
}

func TestDirSizeMB_SkipsSymlinks(t *testing.T) {
	tmp := t.TempDir()
	rootfs := filepath.Join(tmp, "rootfs")
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		t.Fatal(err)
	}
	// Real file: 2 MiB
	if err := os.WriteFile(filepath.Join(rootfs, "real"), make([]byte, 2*1024*1024), 0644); err != nil {
		t.Fatal(err)
	}
	// Symlink to a 10-MiB target outside the tree shouldn't add to the total.
	bigOutside := filepath.Join(tmp, "outside-big")
	if err := os.WriteFile(bigOutside, make([]byte, 10*1024*1024), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(bigOutside, filepath.Join(rootfs, "link")); err != nil {
		t.Fatal(err)
	}
	mb, err := runDirSizeMB(t, tmp, `"rootfs"`)
	if err != nil {
		t.Fatalf("dir_size_mb: %v", err)
	}
	if mb != 2 {
		t.Errorf("got %d MB, want 2 (symlink target should not be counted)", mb)
	}
}

func TestDirSizeMB_NoDestDir(t *testing.T) {
	cfg := &SandboxConfig{}
	thread := NewBuildThread(context.Background(), cfg, &fakeExecer{})
	_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, "test.star",
		`x = dir_size_mb("rootfs")`, BuildPredeclared())
	if err == nil {
		t.Fatal("expected error when destdir is unset")
	}
}
