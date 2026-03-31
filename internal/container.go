package internal

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/YoeDistro/yoe-ng/containers"
)

const (
	containerVersion = "12"
	containerImage   = "yoe-ng"
)

// hostArch returns the host machine architecture in Yoe-NG format.
func hostArch() string {
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

func containerTag(arch string) string {
	host := hostArch()
	if arch == "" || arch == host {
		return containerImage + ":" + containerVersion
	}
	return containerImage + ":" + containerVersion + "-" + arch
}

// Mount describes a bind mount for the container.
type Mount struct {
	Host      string
	Container string
	ReadOnly  bool
}

// ContainerRunConfig configures a single command execution inside the container.
type ContainerRunConfig struct {
	Ctx         context.Context   // optional; nil means background
	Arch        string            // target architecture (empty = host arch)
	Command     string            // shell command to run
	ProjectDir  string            // mounted as /project
	Mounts      []Mount           // additional bind mounts
	Env         map[string]string // environment variables
	Interactive bool              // attach TTY (-it)
	NoUser      bool              // run as root (for losetup/mount)
	Stdout      io.Writer         // override stdout (default: os.Stdout)
	Stderr      io.Writer         // override stderr (default: os.Stderr)
}

var ensureMu sync.Mutex
var ensuredArches = map[string]error{}

// RunInContainer executes a shell command inside the build container.
// The container image is built lazily on first invocation per arch.
func RunInContainer(cfg ContainerRunConfig) error {
	arch := cfg.Arch
	if arch == "" {
		arch = hostArch()
	}

	ensureMu.Lock()
	err, ok := ensuredArches[arch]
	if !ok {
		w := cfg.Stderr
		if w == nil {
			w = os.Stderr
		}
		err = EnsureImage(arch, w)
		ensuredArches[arch] = err
	}
	ensureMu.Unlock()
	if err != nil {
		return fmt.Errorf("container image: %w", err)
	}

	runtime, err := detectRuntime()
	if err != nil {
		return err
	}

	args, err := containerRunArgs(cfg)
	if err != nil {
		return err
	}

	// Assign a unique container name so we can stop it on cancellation.
	// docker run --rm + docker stop is safe: --rm removes the container
	// after it exits, and docker stop gracefully terminates it.
	name := fmt.Sprintf("yoe-%d", rand.Int())
	// Insert --name after "run" (args[0])
	args = append(args[:1], append([]string{"--name", name}, args[1:]...)...)

	args = append(args, cfg.Command)

	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	fmt.Fprintf(stderr, "[yoe] container: %s\n", cfg.Command)

	ctx := cfg.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// When the context is cancelled, stop the container explicitly.
	// exec.CommandContext only kills the docker CLI client, not the
	// container itself.
	done := make(chan struct{})
	if ctx != context.Background() {
		go func() {
			select {
			case <-ctx.Done():
				//nolint:gosec // best-effort cleanup
				exec.Command(runtime, "stop", "-t", "3", name).Run()
			case <-done:
			}
		}()
	}

	cmd := exec.Command(runtime, args...)
	cmd.Stdout = cfg.Stdout
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = stderr
	if cfg.Interactive {
		cmd.Stdin = os.Stdin
	}

	err = cmd.Run()
	close(done)

	// If the context was cancelled, the error is expected.
	if ctx.Err() != nil {
		return fmt.Errorf("build cancelled")
	}
	return err
}

// containerRunArgs builds the docker/podman run arguments (without the
// runtime binary name and without the trailing shell command string).
// The returned args end with "bash" "-c" so the caller only needs to
// append the command string.
func containerRunArgs(cfg ContainerRunConfig) ([]string, error) {
	arch := cfg.Arch
	if arch == "" {
		arch = hostArch()
	}

	args := []string{"run", "--rm", "--privileged"}

	// Add platform for cross-arch containers
	if arch != hostArch() {
		args = append(args, "--platform", "linux/"+arch)
	}

	if !cfg.NoUser {
		u, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("getting current user: %w", err)
		}
		args = append(args, "--user", fmt.Sprintf("%s:%s", u.Uid, u.Gid))
	}

	if cfg.ProjectDir != "" {
		args = append(args, "-v", cfg.ProjectDir+":/project")
	}

	for _, m := range cfg.Mounts {
		mount := m.Host + ":" + m.Container
		if m.ReadOnly {
			mount += ":ro"
		}
		args = append(args, "-v", mount)
	}

	for k, v := range cfg.Env {
		args = append(args, "-e", k+"="+v)
	}

	if cfg.Interactive {
		args = append(args, "-it")
	}

	args = append(args, "-w", "/project")
	args = append(args, containerTag(arch))
	args = append(args, "bash", "-c")

	return args, nil
}

// EnsureImage checks if the versioned container image exists and builds it
// if not. The arch parameter selects the target architecture (empty = host).
// The optional writer receives build progress; if nil, output is discarded
// (safe for TUI mode).
func EnsureImage(arch string, w io.Writer) error {
	runtime, err := detectRuntime()
	if err != nil {
		return err
	}

	tag := containerTag(arch)
	cmd := exec.Command(runtime, "image", "inspect", tag)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Cross-arch: check binfmt_misc first
	host := hostArch()
	if arch != "" && arch != host {
		if err := checkBinfmt(arch); err != nil {
			return err
		}
	}

	if w == nil {
		w = io.Discard
	}
	fmt.Fprintf(w, "[yoe] building container image %s...\n", tag)

	tmpDir, err := os.MkdirTemp("", "yoe-container-build-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(containers.Dockerfile), 0644); err != nil {
		return fmt.Errorf("writing Dockerfile: %w", err)
	}

	if arch != "" && arch != host {
		platform := "linux/" + arch
		cmd = exec.Command(runtime, "buildx", "build",
			"--platform", platform,
			"--load",
			"-t", tag, tmpDir)
	} else {
		cmd = exec.Command(runtime, "build", "-t", tag, tmpDir)
	}
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building container image: %w", err)
	}

	return nil
}

// checkBinfmt verifies that binfmt_misc is registered for the given arch.
func checkBinfmt(arch string) error {
	binfmtName := binfmtArchName(arch)
	path := filepath.Join("/proc/sys/fs/binfmt_misc", binfmtName)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return fmt.Errorf(
		"binfmt_misc not registered for %s.\nRun 'yoe container binfmt' to enable cross-architecture builds",
		arch)
}

func binfmtArchName(arch string) string {
	switch arch {
	case "arm64":
		return "qemu-aarch64"
	case "riscv64":
		return "qemu-riscv64"
	default:
		return "qemu-" + arch
	}
}

// RegisterBinfmt registers QEMU user-mode emulation for foreign architectures
// using the tonistiigi/binfmt Docker image. Requires --privileged.
func RegisterBinfmt(w io.Writer) error {
	runtime, err := detectRuntime()
	if err != nil {
		return err
	}

	fmt.Fprintln(w, "[yoe] registering binfmt_misc handlers...")
	cmd := exec.Command(runtime, "run", "--privileged", "--rm",
		"tonistiigi/binfmt", "--install", "arm64,riscv64")
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("registering binfmt: %w", err)
	}

	fmt.Fprintln(w, "Done. Registered: arm64, riscv64")
	return nil
}

// ContainerVersion returns the container version embedded in this binary.
func ContainerVersion() string {
	return containerVersion
}

func detectRuntime() (string, error) {
	for _, rt := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(rt); err == nil {
			return rt, nil
		}
	}
	return "", fmt.Errorf("neither docker nor podman found — install one to use yoe")
}
