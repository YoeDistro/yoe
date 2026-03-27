package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/YoeDistro/yoe-ng/containers"
)

const (
	// containerVersion is bumped when the Dockerfile changes (i.e., the tool
	// set inside the container changes). The image is tagged yoe-ng:<version>
	// so yoe automatically rebuilds when the version doesn't match.
	containerVersion = "9"

	containerImage = "yoe-ng"
	containerEnv   = "YOE_IN_CONTAINER"
)

func containerTag() string {
	return containerImage + ":" + containerVersion
}

// InContainer returns true if yoe is running inside the build container.
func InContainer() bool {
	return os.Getenv(containerEnv) == "1"
}

// ExecInContainer re-executes the current yoe command inside the build
// container. The host yoe binary is bind-mounted into the container so
// the container image only contains tools, not yoe itself.
//
// The cwd is mounted as /project. If YOE_PROJECT is set, it's passed
// through so the container can find the project root within the mount.
func ExecInContainer(args []string) error {
	hostDir, err := os.Getwd()
	if err != nil {
		return err
	}
	hostDir, err = filepath.Abs(hostDir)
	if err != nil {
		return err
	}

	runtime, err := detectRuntime()
	if err != nil {
		return err
	}

	// Find the running yoe binary
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding yoe binary: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving yoe binary path: %w", err)
	}

	// Determine what to mount. If there's a git repo root above cwd,
	// mount that so relative layer paths (../../layers/...) work.
	mountDir := findGitRoot(hostDir)
	if mountDir == "" {
		mountDir = hostDir
	}

	// Compute the working directory inside the container
	containerWorkDir := "/project"
	if mountDir != hostDir {
		rel, _ := filepath.Rel(mountDir, hostDir)
		containerWorkDir = filepath.Join("/project", rel)
	}

	runArgs := []string{
		"run", "--rm",
		// --privileged provides: bwrap namespaces, losetup/mount for disk
		// images, /dev/kvm for QEMU. Container user is still non-root.
		"--privileged",
		"-v", mountDir + ":/project",
		"-v", exe + ":/usr/local/bin/yoe:ro",
		"-w", containerWorkDir,
		"--entrypoint", "yoe",
	}

	// Attach TTY for interactive commands (yoe run needs QEMU serial console)
	if fileInfo, err := os.Stdin.Stat(); err == nil {
		if (fileInfo.Mode() & os.ModeCharDevice) != 0 {
			runArgs = append(runArgs, "-it")
		}
	}

	// Pass through YOE_PROJECT if set
	if yp := os.Getenv("YOE_PROJECT"); yp != "" {
		runArgs = append(runArgs, "-e", "YOE_PROJECT="+yp)
	}

	// Pass through cache directory
	if cacheDir := os.Getenv("YOE_CACHE"); cacheDir != "" {
		abs, _ := filepath.Abs(cacheDir)
		runArgs = append(runArgs, "-v", abs+":/cache", "-e", "YOE_CACHE=/cache")
	} else {
		home, _ := os.UserHomeDir()
		cacheDir := filepath.Join(home, ".cache", "yoe-ng")
		os.MkdirAll(cacheDir, 0755)
		runArgs = append(runArgs, "-v", cacheDir+":/cache", "-e", "YOE_CACHE=/cache")
	}

	// Note: running as root inside the container (needed for losetup/mount
	// during disk image creation). File ownership on the bind mount follows
	// the host filesystem.

	runArgs = append(runArgs, containerTag())
	runArgs = append(runArgs, args...)

	fmt.Fprintf(os.Stderr, "[yoe] running in container: %s %s\n", runtime, strings.Join(args, " "))

	cmd := exec.Command(runtime, runArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// EnsureImage checks if the versioned container image exists and builds it
// if not. Since the container only contains tools (not yoe), it only needs
// rebuilding when the tool set changes (version bump in Dockerfile).
func EnsureImage() error {
	runtime, err := detectRuntime()
	if err != nil {
		return err
	}

	tag := containerTag()
	cmd := exec.Command(runtime, "image", "inspect", tag)
	if err := cmd.Run(); err == nil {
		return nil // correct version exists
	}

	fmt.Fprintf(os.Stderr, "[yoe] building container image %s...\n", tag)

	// Create a temp build context with just the Dockerfile
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

// findGitRoot walks up from dir to find the nearest .git directory.
func findGitRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func detectRuntime() (string, error) {
	for _, rt := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(rt); err == nil {
			return rt, nil
		}
	}
	return "", fmt.Errorf("neither docker nor podman found — install one to use yoe")
}
