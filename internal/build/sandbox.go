package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yoe "github.com/YoeDistro/yoe-ng/internal"
	"github.com/YoeDistro/yoe-ng/internal/resolve"
)

// SandboxConfig defines the bubblewrap sandbox for a unit build.
type SandboxConfig struct {
	Ctx        context.Context
	BuildRoot  string
	SrcDir     string
	DestDir    string
	Sysroot    string
	Env        map[string]string
	ProjectDir string
	Stdout     io.Writer // build output (nil = os.Stdout)
	Stderr     io.Writer // build errors (nil = os.Stderr)
}

// RunInSandbox executes a command inside a bubblewrap sandbox within the
// build container.
func RunInSandbox(cfg *SandboxConfig, command string) error {
	bwrapCmd := bwrapCommand(cfg, command)
	mounts := containerMountsForBuild(cfg)

	return yoe.RunInContainer(yoe.ContainerRunConfig{
		Ctx:        cfg.Ctx,
		Command:    bwrapCmd,
		ProjectDir: cfg.ProjectDir,
		Mounts:     mounts,
		Stdout:     cfg.Stdout,
		Stderr:     cfg.Stderr,
	})
}

// RunSimple executes a command directly in the container (no bwrap sandbox).
// Used for Stage 0 bootstrap where we use the container's Alpine toolchain.
func RunSimple(cfg *SandboxConfig, command string) error {
	var envExports []string
	for k, v := range cfg.Env {
		envExports = append(envExports, fmt.Sprintf("export %s=%q", k, v))
	}
	fullCmd := strings.Join(envExports, "; ")
	if fullCmd != "" {
		fullCmd += "; "
	}
	fullCmd += "cd /build/src && " + command

	mounts := containerMountsForBuild(cfg)

	return yoe.RunInContainer(yoe.ContainerRunConfig{
		Ctx:        cfg.Ctx,
		Command:    fullCmd,
		ProjectDir: cfg.ProjectDir,
		Mounts:     mounts,
		Stdout:     cfg.Stdout,
		Stderr:     cfg.Stderr,
	})
}

func bwrapCommand(cfg *SandboxConfig, command string) string {
	var parts []string
	parts = append(parts, "bwrap", "--die-with-parent")

	if cfg.BuildRoot != "" {
		parts = append(parts, "--bind", cfg.BuildRoot, "/")
	} else {
		parts = append(parts, "--bind", "/", "/")
	}

	if cfg.Sysroot != "" {
		parts = append(parts, "--ro-bind", "/build/sysroot", "/build/sysroot")
	}

	parts = append(parts,
		"--bind", "/build/src", "/build/src",
		"--bind", "/build/destdir", "/build/destdir",
		"--dev-bind", "/dev", "/dev",
		"--ro-bind", "/proc", "/proc",
		"--tmpfs", "/tmp",
		"--chdir", "/build/src",
	)

	var envExports []string
	for k, v := range cfg.Env {
		envExports = append(envExports, fmt.Sprintf("export %s=%q", k, v))
	}
	envStr := strings.Join(envExports, "; ")
	fullCmd := envStr
	if fullCmd != "" {
		fullCmd += "; "
	}
	fullCmd += command

	parts = append(parts, "--", "bash", "-c", shellQuote(fullCmd))
	return strings.Join(parts, " ")
}

// BwrapShellCommand returns a bwrap command string that launches an
// interactive bash shell with the given sandbox config's mounts and env.
func BwrapShellCommand(cfg *SandboxConfig) string {
	var parts []string
	parts = append(parts, "bwrap", "--die-with-parent")

	if cfg.BuildRoot != "" {
		parts = append(parts, "--bind", cfg.BuildRoot, "/")
	} else {
		parts = append(parts, "--bind", "/", "/")
	}

	if cfg.Sysroot != "" {
		parts = append(parts, "--ro-bind", "/build/sysroot", "/build/sysroot")
	}

	parts = append(parts,
		"--bind", "/build/src", "/build/src",
		"--bind", "/build/destdir", "/build/destdir",
		"--dev-bind", "/dev", "/dev",
		"--ro-bind", "/proc", "/proc",
		"--tmpfs", "/tmp",
		"--chdir", "/build/src",
	)

	// Export env vars then exec interactive bash
	var envExports []string
	for k, v := range cfg.Env {
		envExports = append(envExports, fmt.Sprintf("export %s=%q", k, v))
	}
	envStr := strings.Join(envExports, "; ")
	if envStr != "" {
		envStr += "; "
	}
	envStr += "exec bash"

	parts = append(parts, "--", "bash", "-c", shellQuote(envStr))
	return strings.Join(parts, " ")
}

// shellQuote wraps a string in single quotes for safe embedding in a
// shell command. Single quotes inside the string are escaped.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func containerMountsForBuild(cfg *SandboxConfig) []yoe.Mount {
	var mounts []yoe.Mount

	if cfg.SrcDir != "" {
		mounts = append(mounts, yoe.Mount{
			Host: cfg.SrcDir, Container: "/build/src",
		})
	}
	if cfg.DestDir != "" {
		mounts = append(mounts, yoe.Mount{
			Host: cfg.DestDir, Container: "/build/destdir",
		})
	}
	if cfg.Sysroot != "" {
		mounts = append(mounts, yoe.Mount{
			Host: cfg.Sysroot, Container: "/build/sysroot", ReadOnly: true,
		})
	}

	return mounts
}

// StageSysroot hardlinks a unit's destdir into its sysroot staging area
// so downstream units can include it in their per-unit sysroots.
func StageSysroot(destDir, buildDir string) error {
	stageDir := filepath.Join(buildDir, "sysroot-stage")
	os.RemoveAll(stageDir)
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return err
	}
	cmd := exec.Command("cp", "-al", destDir+"/.", stageDir+"/")
	if err := cmd.Run(); err != nil {
		// Fall back to regular copy if hardlinks fail (e.g., cross-device)
		cmd = exec.Command("cp", "-a", destDir+"/.", stageDir+"/")
		return cmd.Run()
	}
	return nil
}

// AssembleSysroot merges the sysroot-stage dirs of all transitive deps
// into a unit's private sysroot.
func AssembleSysroot(sysrootDir string, dag *resolve.DAG, unit string, projectDir string) error {
	os.RemoveAll(sysrootDir)
	if err := os.MkdirAll(sysrootDir, 0755); err != nil {
		return err
	}
	for _, dep := range dag.TransitiveDeps(unit) {
		stageDir := filepath.Join(UnitBuildDir(projectDir, dep), "sysroot-stage")
		if _, err := os.Stat(stageDir); err != nil {
			continue // dep has no staged output (e.g., image)
		}
		cmd := exec.Command("cp", "-al", stageDir+"/.", sysrootDir+"/")
		if err := cmd.Run(); err != nil {
			// Fall back to regular copy
			cmd = exec.Command("cp", "-a", stageDir+"/.", sysrootDir+"/")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("merging sysroot from %s: %w", dep, err)
			}
		}
	}
	return nil
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

// UnitBuildDir returns the build directory for a unit.
func UnitBuildDir(projectDir, unitName string) string {
	return filepath.Join(projectDir, "build", unitName)
}
