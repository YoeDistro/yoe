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
	// Sysroot is the shared build sysroot containing installed deps.
	// Overlaid onto /usr so recipes can find deps' headers and libraries.
	Sysroot string
	// Env is the build environment variables
	Env map[string]string
}

// RunInSandbox executes a command inside a bubblewrap sandbox.
func RunInSandbox(cfg *SandboxConfig, command string) error {
	args := []string{
		"--die-with-parent",
	}

	// Mount the container root read-write (we're already inside Docker,
	// which provides the isolation). Bwrap adds per-recipe PID isolation
	// and prevents accidental host contamination.
	if cfg.BuildRoot != "" {
		args = append(args, "--bind", cfg.BuildRoot, "/")
	} else {
		args = append(args, "--bind", "/", "/")
	}

	// Mount sysroot at /build/sysroot — contains deps' headers/libraries.
	// Build environment vars (CFLAGS, LDFLAGS, PKG_CONFIG_PATH) point here.
	if cfg.Sysroot != "" {
		args = append(args, "--ro-bind", cfg.Sysroot, "/build/sysroot")
	}

	// Mount source and destdir into /build
	args = append(args,
		"--bind", cfg.SrcDir, "/build/src",
		"--bind", cfg.DestDir, "/build/destdir",
		"--dev-bind", "/dev", "/dev",
		"--ro-bind", "/proc", "/proc",
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

// SysrootDir returns the shared build sysroot path for a project.
func SysrootDir(projectDir string) string {
	return filepath.Join(projectDir, "build", "sysroot")
}

// InstallToSysroot copies a recipe's destdir contents into the shared sysroot.
// This makes the recipe's headers, libraries, and pkg-config files available
// to subsequent recipe builds.
func InstallToSysroot(destDir, sysrootDir string) error {
	if err := os.MkdirAll(sysrootDir, 0755); err != nil {
		return err
	}
	cmd := exec.Command("cp", "-a", destDir+"/.", sysrootDir+"/")
	return cmd.Run()
}

// HasBwrap returns true if bubblewrap is available and can create namespaces.
// Inside Docker containers, bwrap may be installed but unable to create user
// namespaces, so we test with a trivial command.
func HasBwrap() bool {
	if _, err := exec.LookPath("bwrap"); err != nil {
		return false
	}
	// Test if bwrap can actually create a namespace
	cmd := exec.Command("bwrap", "--ro-bind", "/", "/", "--dev", "/dev", "true")
	return cmd.Run() == nil
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
