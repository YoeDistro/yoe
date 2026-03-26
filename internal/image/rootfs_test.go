package image

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

func TestAssemble(t *testing.T) {
	projectDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	// Create a fake local repo with nothing (apk won't be available in tests)
	os.MkdirAll(filepath.Join(projectDir, "build", "repo"), 0755)

	// Create an overlay
	overlayDir := filepath.Join(projectDir, "overlays", "etc", "myapp")
	os.MkdirAll(overlayDir, 0755)
	os.WriteFile(filepath.Join(overlayDir, "config.toml"), []byte("key = \"value\"\n"), 0644)

	recipe := &yoestar.Recipe{
		Name:     "test-image",
		Version:  "1.0.0",
		Class:    "image",
		Packages: []string{"openssh", "myapp"},
		Hostname: "yoe-test",
		Timezone: "UTC",
		Locale:   "en_US.UTF-8",
		Services: []string{"sshd", "myapp"},
	}

	proj := &yoestar.Project{
		Name: "test",
	}

	var buf bytes.Buffer
	if err := Assemble(recipe, proj, projectDir, outputDir, &buf); err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	output := buf.String()

	// Check hostname was set
	hostname, _ := os.ReadFile(filepath.Join(outputDir, "rootfs", "etc", "hostname"))
	if strings.TrimSpace(string(hostname)) != "yoe-test" {
		t.Errorf("hostname = %q, want %q", string(hostname), "yoe-test")
	}

	// Check timezone symlink
	localtime := filepath.Join(outputDir, "rootfs", "etc", "localtime")
	link, err := os.Readlink(localtime)
	if err != nil {
		t.Errorf("localtime symlink: %v", err)
	} else if link != "/usr/share/zoneinfo/UTC" {
		t.Errorf("localtime = %q, want UTC", link)
	}

	// Check services enabled
	sshLink := filepath.Join(outputDir, "rootfs", "etc", "systemd", "system",
		"multi-user.target.wants", "sshd.service")
	if _, err := os.Lstat(sshLink); os.IsNotExist(err) {
		t.Error("sshd service not enabled")
	}

	// Check overlay was applied
	overlayFile := filepath.Join(outputDir, "rootfs", "etc", "myapp", "config.toml")
	if _, err := os.Stat(overlayFile); os.IsNotExist(err) {
		t.Error("overlay file not applied")
	}

	// Check disk image was generated
	imgPath := filepath.Join(outputDir, "test-image.img")
	if _, err := os.Stat(imgPath); os.IsNotExist(err) {
		t.Error("disk image not generated")
	}

	// Check output messages
	if !strings.Contains(output, "hostname") {
		t.Error("output should mention hostname")
	}
	if !strings.Contains(output, "sshd") {
		t.Error("output should mention sshd service")
	}
}

func TestApplyConfig_Empty(t *testing.T) {
	rootfs := filepath.Join(t.TempDir(), "rootfs")
	os.MkdirAll(rootfs, 0755)

	recipe := &yoestar.Recipe{Name: "empty"}
	var buf bytes.Buffer
	if err := applyConfig(rootfs, recipe, &buf); err != nil {
		t.Fatalf("applyConfig: %v", err)
	}
	// Should succeed with no config to apply
}
