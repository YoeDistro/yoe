package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yoe "github.com/YoeDistro/yoe-ng/internal"
)

// SandboxConfig defines the bubblewrap sandbox for a recipe build.
type SandboxConfig struct {
	BuildRoot  string
	SrcDir     string
	DestDir    string
	Sysroot    string
	Env        map[string]string
	ProjectDir string
}

// RunInSandbox executes a command inside a bubblewrap sandbox within the
// build container.
func RunInSandbox(cfg *SandboxConfig, command string) error {
	bwrapCmd := bwrapCommand(cfg, command)
	mounts := containerMountsForBuild(cfg)

	return yoe.RunInContainer(yoe.ContainerRunConfig{
		Command:    bwrapCmd,
		ProjectDir: cfg.ProjectDir,
		Mounts:     mounts,
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
		Command:    fullCmd,
		ProjectDir: cfg.ProjectDir,
		Mounts:     mounts,
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

	parts = append(parts, "--", "sh", "-c", fullCmd)
	return strings.Join(parts, " ")
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

// SysrootDir returns the shared build sysroot path for a project.
func SysrootDir(projectDir string) string {
	return filepath.Join(projectDir, "build", "sysroot")
}

// InstallToSysroot copies a recipe's destdir contents into the shared sysroot.
func InstallToSysroot(destDir, sysrootDir string) error {
	if err := os.MkdirAll(sysrootDir, 0755); err != nil {
		return err
	}
	cmd := exec.Command("cp", "-a", destDir+"/.", sysrootDir+"/")
	return cmd.Run()
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
