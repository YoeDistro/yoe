package build

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YoeDistro/yoe-ng/internal/resolve"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

func TestBuildCommands_Package(t *testing.T) {
	unit := &yoestar.Unit{
		Name:  "test",
		Class: "unit",
		Build: []string{"make", "make install"},
	}
	cmds := buildCommands(unit)
	if len(cmds) != 2 {
		t.Errorf("got %d commands, want 2", len(cmds))
	}
}

func TestBuildCommands_Autotools(t *testing.T) {
	unit := &yoestar.Unit{
		Name:          "test",
		Class:         "autotools",
		ConfigureArgs: []string{"--with-ssl"},
	}
	cmds := buildCommands(unit)
	if len(cmds) != 3 {
		t.Errorf("got %d commands, want 3", len(cmds))
	}
	if !strings.Contains(cmds[0], "--with-ssl") {
		t.Errorf("configure should include --with-ssl: %s", cmds[0])
	}
}

func TestBuildCommands_CMake(t *testing.T) {
	unit := &yoestar.Unit{
		Name:  "test",
		Class: "cmake",
	}
	cmds := buildCommands(unit)
	if len(cmds) != 3 {
		t.Errorf("got %d commands, want 3", len(cmds))
	}
	if !strings.Contains(cmds[0], "cmake -B build") {
		t.Errorf("first command should be cmake: %s", cmds[0])
	}
}

func TestBuildCommands_Go(t *testing.T) {
	unit := &yoestar.Unit{
		Name:      "myapp",
		Class:     "go",
		GoPackage: "./cmd/myapp",
	}
	cmds := buildCommands(unit)
	if len(cmds) != 1 {
		t.Errorf("got %d commands, want 1", len(cmds))
	}
	if !strings.Contains(cmds[0], "go build") {
		t.Errorf("command should be go build: %s", cmds[0])
	}
	if !strings.Contains(cmds[0], "./cmd/myapp") {
		t.Errorf("command should include package path: %s", cmds[0])
	}
}

func TestBuildCommands_Image(t *testing.T) {
	unit := &yoestar.Unit{
		Name:  "base-image",
		Class: "image",
	}
	cmds := buildCommands(unit)
	if cmds != nil {
		t.Errorf("image should have no build commands, got %v", cmds)
	}
}

func TestDryRun(t *testing.T) {
	proj := &yoestar.Project{
		Name: "test",
		Units: map[string]*yoestar.Unit{
			"zlib":    {Name: "zlib", Version: "1.3", Class: "unit", Build: []string{"make"}},
			"openssh": {Name: "openssh", Version: "9.6", Class: "unit", Deps: []string{"zlib"}, Build: []string{"make"}},
		},
	}

	var buf bytes.Buffer
	opts := Options{
		DryRun:     true,
		ProjectDir: t.TempDir(),
		Arch:       "arm64",
	}

	if err := BuildUnits(proj, nil, opts, &buf); err != nil {
		t.Fatalf("BuildUnits dry run: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "zlib") {
		t.Error("dry run should list zlib")
	}
	if !strings.Contains(output, "openssh") {
		t.Error("dry run should list openssh")
	}
}

func TestCacheMarker(t *testing.T) {
	dir := t.TempDir()
	name := "test-unit"
	hash := "abc123def456"

	arch := "x86_64"

	// Not cached initially
	if IsBuildCached(dir, arch, name, hash) {
		t.Error("should not be cached initially")
	}

	// Write marker
	writeCacheMarker(dir, arch, name, hash)

	// Now cached
	if !IsBuildCached(dir, arch, name, hash) {
		t.Error("should be cached after writing marker")
	}

	// Different hash not cached
	if IsBuildCached(dir, arch, name, "different") {
		t.Error("different hash should not be cached")
	}
}

func TestFilterBuildOrder(t *testing.T) {
	proj := &yoestar.Project{
		Units: map[string]*yoestar.Unit{
			"a": {Name: "a"},
			"b": {Name: "b", Deps: []string{"a"}},
			"c": {Name: "c", Deps: []string{"b"}},
			"d": {Name: "d"},
		},
	}

	dag, _ := resolve.BuildDAG(proj)
	order, _ := dag.TopologicalSort()

	filtered, err := filterBuildOrder(dag, order, []string{"c"})
	if err != nil {
		t.Fatalf("filterBuildOrder: %v", err)
	}

	// c depends on b depends on a — should include all three but not d
	if len(filtered) != 3 {
		t.Errorf("got %d units, want 3 (a, b, c)", len(filtered))
	}

	has := make(map[string]bool)
	for _, n := range filtered {
		has[n] = true
	}
	if !has["a"] || !has["b"] || !has["c"] {
		t.Errorf("filtered = %v, should contain a, b, c", filtered)
	}
	if has["d"] {
		t.Error("filtered should not contain d")
	}
}

func TestBuildUnits_WithDeps(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("requires --privileged container with user namespace support")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		if _, err := exec.LookPath("podman"); err != nil {
			t.Skip("docker/podman not available")
		}
	}

	// Create a project with units that have trivial build steps
	projectDir := t.TempDir()

	proj := &yoestar.Project{
		Name: "test",
		Units: map[string]*yoestar.Unit{
			"hello": {
				Name:    "hello",
				Version: "1.0",
				Class:   "package",
				Build:   []string{"echo built > built.txt"},
			},
		},
	}

	// Create source directory with a file (simulating prepared source)
	srcDir := filepath.Join(projectDir, "build", "hello.x86_64", "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "Makefile"), []byte("all:\n\techo hello\n"), 0644)

	// Init git so Prepare doesn't try to fetch
	run(t, srcDir, "git", "init")
	run(t, srcDir, "git", "config", "user.email", "test@test.com")
	run(t, srcDir, "git", "config", "user.name", "Test")
	run(t, srcDir, "git", "add", "-A")
	run(t, srcDir, "git", "commit", "-m", "upstream")
	run(t, srcDir, "git", "tag", "upstream")
	// Add a local commit so Prepare treats it as dev mode
	os.WriteFile(filepath.Join(srcDir, "local.txt"), []byte("local\n"), 0644)
	run(t, srcDir, "git", "add", "-A")
	run(t, srcDir, "git", "commit", "-m", "local")

	var buf bytes.Buffer
	opts := Options{
		ProjectDir: projectDir,
		Arch:       "x86_64",
	}

	if err := BuildUnits(proj, []string{"hello"}, opts, &buf); err != nil {
		t.Fatalf("BuildUnits: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "hello") {
		t.Errorf("output should mention hello: %s", output)
	}
	if !strings.Contains(output, "done") {
		t.Errorf("output should mention done: %s", output)
	}

	// Verify cache marker was written
	if !IsBuildCached(projectDir, "x86_64", "hello", "") {
		// The hash won't be "" — just verify the marker file exists
		markerDir := filepath.Join(projectDir, "build", "hello.x86_64")
		entries, _ := os.ReadDir(markerDir)
		found := false
		for _, e := range entries {
			if e.Name() == ".yoe-hash" {
				found = true
			}
		}
		if !found {
			t.Error("cache marker not written")
		}
	}
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}
