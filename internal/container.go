package internal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sync"

	"github.com/YoeDistro/yoe-ng/containers"
)

const (
	containerVersion = "10"
	containerImage   = "yoe-ng"
)

func containerTag() string {
	return containerImage + ":" + containerVersion
}

// Mount describes a bind mount for the container.
type Mount struct {
	Host      string
	Container string
	ReadOnly  bool
}

// ContainerRunConfig configures a single command execution inside the container.
type ContainerRunConfig struct {
	Command     string            // shell command to run
	ProjectDir  string            // mounted as /project
	Mounts      []Mount           // additional bind mounts
	Env         map[string]string // environment variables
	Interactive bool              // attach TTY (-it)
	NoUser      bool              // run as root (for losetup/mount)
	Stdout      io.Writer         // override stdout (default: os.Stdout)
	Stderr      io.Writer         // override stderr (default: os.Stderr)
}

var ensureOnce sync.Once
var ensureErr error

// RunInContainer executes a shell command inside the build container.
// The container image is built lazily on first invocation.
func RunInContainer(cfg ContainerRunConfig) error {
	ensureOnce.Do(func() {
		ensureErr = EnsureImage()
	})
	if ensureErr != nil {
		return fmt.Errorf("container image: %w", ensureErr)
	}

	runtime, err := detectRuntime()
	if err != nil {
		return err
	}

	args, err := containerRunArgs(cfg)
	if err != nil {
		return err
	}

	args = append(args, cfg.Command)

	fmt.Fprintf(os.Stderr, "[yoe] container: %s\n", cfg.Command)

	cmd := exec.Command(runtime, args...)
	cmd.Stdout = cfg.Stdout
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = cfg.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	if cfg.Interactive {
		cmd.Stdin = os.Stdin
	}

	return cmd.Run()
}

// containerRunArgs builds the docker/podman run arguments (without the
// runtime binary name and without the trailing shell command string).
// The returned args end with "sh" "-c" so the caller only needs to
// append the command string.
func containerRunArgs(cfg ContainerRunConfig) ([]string, error) {
	args := []string{"run", "--rm", "--privileged"}

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
	args = append(args, containerTag())
	args = append(args, "sh", "-c")

	return args, nil
}

// EnsureImage checks if the versioned container image exists and builds it
// if not.
func EnsureImage() error {
	runtime, err := detectRuntime()
	if err != nil {
		return err
	}

	tag := containerTag()
	cmd := exec.Command(runtime, "image", "inspect", tag)
	if err := cmd.Run(); err == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "[yoe] building container image %s...\n", tag)

	tmpDir, err := os.MkdirTemp("", "yoe-container-build-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(containers.Dockerfile), 0644); err != nil {
		return fmt.Errorf("writing Dockerfile: %w", err)
	}

	cmd = exec.Command(runtime, "build", "-t", tag, tmpDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building container image: %w", err)
	}

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
