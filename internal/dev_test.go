package internal

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevExtract(t *testing.T) {
	// Create a temp project with a unit
	dir := t.TempDir()
	setupDevTestProject(t, dir)

	// Create a fake build/openssh/src git repo simulating a fetched source
	srcDir := filepath.Join(dir, "build", "openssh", "src")
	os.MkdirAll(srcDir, 0755)

	// Init git repo with upstream content
	run(t, srcDir, "git", "init")
	run(t, srcDir, "git", "config", "user.email", "test@test.com")
	run(t, srcDir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(srcDir, "main.c"), []byte("int main() { return 0; }\n"), 0644)
	run(t, srcDir, "git", "add", "-A")
	run(t, srcDir, "git", "commit", "-m", "upstream source")
	run(t, srcDir, "git", "tag", "upstream")

	// Make a local change (simulating developer edits)
	os.WriteFile(filepath.Join(srcDir, "main.c"), []byte("int main() { return 42; }\n"), 0644)
	run(t, srcDir, "git", "add", "-A")
	run(t, srcDir, "git", "commit", "-m", "fix return value")

	// Extract patches
	var buf bytes.Buffer
	if err := DevExtract(dir, "openssh", &buf); err != nil {
		t.Fatalf("DevExtract: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1 patch") {
		t.Errorf("output should mention 1 patch, got: %s", output)
	}

	// Verify patch file was created
	patches, _ := filepath.Glob(filepath.Join(dir, "patches", "openssh", "*.patch"))
	if len(patches) != 1 {
		t.Errorf("expected 1 patch file, got %d", len(patches))
	}

	// Verify patch content
	if len(patches) > 0 {
		content, _ := os.ReadFile(patches[0])
		if !strings.Contains(string(content), "return 42") {
			t.Error("patch should contain the change")
		}
	}
}

func TestDevExtract_NoCommits(t *testing.T) {
	dir := t.TempDir()
	setupDevTestProject(t, dir)

	srcDir := filepath.Join(dir, "build", "openssh", "src")
	os.MkdirAll(srcDir, 0755)

	run(t, srcDir, "git", "init")
	run(t, srcDir, "git", "config", "user.email", "test@test.com")
	run(t, srcDir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(srcDir, "main.c"), []byte("int main() {}\n"), 0644)
	run(t, srcDir, "git", "add", "-A")
	run(t, srcDir, "git", "commit", "-m", "upstream")
	run(t, srcDir, "git", "tag", "upstream")

	var buf bytes.Buffer
	if err := DevExtract(dir, "openssh", &buf); err != nil {
		t.Fatalf("DevExtract: %v", err)
	}

	if !strings.Contains(buf.String(), "No local commits") {
		t.Errorf("should report no local commits, got: %s", buf.String())
	}
}

func TestDevDiff(t *testing.T) {
	dir := t.TempDir()
	setupDevTestProject(t, dir)

	srcDir := filepath.Join(dir, "build", "openssh", "src")
	os.MkdirAll(srcDir, 0755)

	run(t, srcDir, "git", "init")
	run(t, srcDir, "git", "config", "user.email", "test@test.com")
	run(t, srcDir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(srcDir, "main.c"), []byte("int main() {}\n"), 0644)
	run(t, srcDir, "git", "add", "-A")
	run(t, srcDir, "git", "commit", "-m", "upstream")
	run(t, srcDir, "git", "tag", "upstream")

	os.WriteFile(filepath.Join(srcDir, "main.c"), []byte("int main() { return 1; }\n"), 0644)
	run(t, srcDir, "git", "add", "-A")
	run(t, srcDir, "git", "commit", "-m", "my change")

	var buf bytes.Buffer
	if err := DevDiff(dir, "openssh", &buf); err != nil {
		t.Fatalf("DevDiff: %v", err)
	}

	if !strings.Contains(buf.String(), "my change") {
		t.Errorf("should show commit message, got: %s", buf.String())
	}
}

func TestDevStatus(t *testing.T) {
	dir := t.TempDir()
	setupDevTestProject(t, dir)

	// openssh: has local commits
	srcDir := filepath.Join(dir, "build", "openssh", "src")
	os.MkdirAll(srcDir, 0755)
	run(t, srcDir, "git", "init")
	run(t, srcDir, "git", "config", "user.email", "test@test.com")
	run(t, srcDir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(srcDir, "main.c"), []byte("orig\n"), 0644)
	run(t, srcDir, "git", "add", "-A")
	run(t, srcDir, "git", "commit", "-m", "upstream")
	run(t, srcDir, "git", "tag", "upstream")
	os.WriteFile(filepath.Join(srcDir, "main.c"), []byte("changed\n"), 0644)
	run(t, srcDir, "git", "add", "-A")
	run(t, srcDir, "git", "commit", "-m", "local fix")

	var buf bytes.Buffer
	if err := DevStatus(dir, &buf); err != nil {
		t.Fatalf("DevStatus: %v", err)
	}

	if !strings.Contains(buf.String(), "openssh") {
		t.Errorf("should list openssh as modified, got: %s", buf.String())
	}
}

func setupDevTestProject(t *testing.T, dir string) {
	t.Helper()
	// Create a minimal project with an openssh unit
	os.MkdirAll(filepath.Join(dir, "units"), 0755)
	os.MkdirAll(filepath.Join(dir, "machines"), 0755)

	os.WriteFile(filepath.Join(dir, "PROJECT.star"), []byte(
		`project(name = "test", version = "0.1.0")`+"\n",
	), 0644)

	os.WriteFile(filepath.Join(dir, "units", "openssh.star"), []byte(
		`unit(name = "openssh", version = "9.6p1", source = "https://example.com/openssh.tar.gz", build = ["make"])`+"\n",
	), 0644)
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}
