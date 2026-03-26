package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SandboxConfig defines the bubblewrap sandbox for a recipe build.
type SandboxConfig struct {
	// BuildRoot is the Tier 1 build root (ro-bind mounted as /)
	BuildRoot string
	// SrcDir is the recipe source directory (bind mounted as /build/src)
	SrcDir string
	// DestDir is the staging directory (bind mounted as /build/destdir)
	DestDir string
	// Env is the build environment variables
	Env map[string]string
}

// RunInSandbox executes a command inside a bubblewrap sandbox.
func RunInSandbox(cfg *SandboxConfig, command string) error {
	args := []string{
		"--die-with-parent",
		"--unshare-pid",
	}

	// Mount build root read-only as base filesystem
	if cfg.BuildRoot != "" {
		args = append(args, "--ro-bind", cfg.BuildRoot, "/")
	} else {
		// No build root yet — bind the host root read-only
		args = append(args, "--ro-bind", "/", "/")
	}

	// Mount source and destdir read-write
	args = append(args,
		"--bind", cfg.SrcDir, "/build/src",
		"--bind", cfg.DestDir, "/build/destdir",
		"--dev", "/dev",
		"--proc", "/proc",
		"--tmpfs", "/tmp",
	)

	// Set working directory to source
	args = append(args, "--chdir", "/build/src")

	// Build the shell command with environment
	var envExports []string
	for k, v := range cfg.Env {
		envExports = append(envExports, fmt.Sprintf("export %s=%q", k, v))
	}
	envStr := strings.Join(envExports, "; ")
	fullCmd := envStr + "; " + command

	args = append(args, "--", "sh", "-c", fullCmd)

	cmd := exec.Command("bwrap", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sandbox execution failed: %w", err)
	}
	return nil
}

// RunSimple executes a command directly (no sandbox) for when bwrap
// is not available (e.g., initial development, or inside the container
// where the container itself provides isolation).
func RunSimple(srcDir, destDir string, env map[string]string, command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build step failed: %w", err)
	}
	return nil
}

// HasBwrap returns true if bubblewrap is available.
func HasBwrap() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// NProc returns the number of available CPU cores.
func NProc() string {
	out, err := exec.Command("nproc").Output()
	if err != nil {
		return "1"
	}
	return strings.TrimSpace(string(out))
}

// Arch returns the current machine architecture in Yoe-NG format.
func Arch() string {
	out, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return "x86_64"
	}
	arch := strings.TrimSpace(string(out))
	switch arch {
	case "aarch64":
		return "arm64"
	default:
		return arch
	}
}

// RecipeBuildDir returns the build directory for a recipe.
func RecipeBuildDir(projectDir, recipeName string) string {
	return filepath.Join(projectDir, "build", recipeName)
}
