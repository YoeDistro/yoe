package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	containerImage = "yoe-ng"
	containerEnv   = "YOE_IN_CONTAINER"
)

// InContainer returns true if yoe is running inside the build container.
func InContainer() bool {
	return os.Getenv(containerEnv) == "1"
}

// ExecInContainer re-executes the current yoe command inside the build
// container with the project directory mounted. This replaces the current
// process on success (exec).
func ExecInContainer(args []string) error {
	// Find project root (or use cwd)
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Resolve to absolute path
	projectDir, err = filepath.Abs(projectDir)
	if err != nil {
		return err
	}

	// Detect container runtime
	runtime, err := detectRuntime()
	if err != nil {
		return err
	}

	// Build docker/podman run command
	runArgs := []string{
		"run", "--rm",
		"-v", projectDir + ":/project",
		"-w", "/project",
	}

	// Pass through cache directory if set
	if cacheDir := os.Getenv("YOE_CACHE"); cacheDir != "" {
		abs, _ := filepath.Abs(cacheDir)
		runArgs = append(runArgs, "-v", abs+":/cache", "-e", "YOE_CACHE=/cache")
	} else {
		// Default: mount ~/.cache/yoe-ng
		home, _ := os.UserHomeDir()
		cacheDir = filepath.Join(home, ".cache", "yoe-ng")
		os.MkdirAll(cacheDir, 0755)
		runArgs = append(runArgs, "-v", cacheDir+":/cache", "-e", "YOE_CACHE=/cache")
	}

	// Pass through user/group for file ownership
	runArgs = append(runArgs, "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()))

	// Image name
	runArgs = append(runArgs, containerImage)

	// Append the original yoe arguments
	runArgs = append(runArgs, args...)

	fmt.Fprintf(os.Stderr, "[yoe] running in container: %s %s\n", runtime, strings.Join(args, " "))

	// Exec replaces the current process
	cmd := exec.Command(runtime, runArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// EnsureImage builds the container image if it doesn't exist.
func EnsureImage() error {
	runtime, err := detectRuntime()
	if err != nil {
		return err
	}

	// Check if image exists
	cmd := exec.Command(runtime, "image", "inspect", containerImage)
	if err := cmd.Run(); err == nil {
		return nil // image exists
	}

	// Find the Dockerfile
	dockerfile := findDockerfile()
	if dockerfile == "" {
		return fmt.Errorf("container image %q not found and no Dockerfile available\n"+
			"Run: docker build -f containers/Dockerfile.build -t yoe-ng .", containerImage)
	}

	fmt.Fprintf(os.Stderr, "[yoe] building container image %s...\n", containerImage)

	// Build context is the repo root (parent of containers/)
	contextDir := filepath.Dir(filepath.Dir(dockerfile))
	cmd = exec.Command(runtime, "build", "-f", dockerfile, "-t", containerImage, contextDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func detectRuntime() (string, error) {
	for _, rt := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(rt); err == nil {
			return rt, nil
		}
	}
	return "", fmt.Errorf("neither docker nor podman found — install one to use yoe")
}

func findDockerfile() string {
	// Check relative to the yoe binary location
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exeDir := filepath.Dir(exe)

	candidates := []string{
		filepath.Join(exeDir, "containers", "Dockerfile.build"),
		filepath.Join(exeDir, "..", "containers", "Dockerfile.build"),
		"containers/Dockerfile.build",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}
