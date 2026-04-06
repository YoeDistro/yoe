package internal

import (
	"fmt"
	"os/user"
	"testing"
)

func TestContainerRunArgs_Basic(t *testing.T) {
	cfg := ContainerRunConfig{
		Command:    "echo hello",
		Image:      "yoe-ng/toolchain-musl:15-x86_64",
		ProjectDir: "/home/user/myproject",
	}

	args, err := containerRunArgs(cfg)
	if err != nil {
		t.Fatalf("containerRunArgs: %v", err)
	}

	assertContains(t, args, "--rm")
	assertContains(t, args, "--privileged")

	u, _ := user.Current()
	assertContains(t, args, "--user")
	assertContains(t, args, fmt.Sprintf("%s:%s", u.Uid, u.Gid))

	assertContains(t, args, "-v")
	assertContains(t, args, "/home/user/myproject:/project")

	last3 := args[len(args)-3:]
	if last3[0] != "yoe-ng/toolchain-musl:15-x86_64" {
		t.Errorf("expected image tag %q, got %q", "yoe-ng/toolchain-musl:15-x86_64", last3[0])
	}
	if last3[1] != "bash" || last3[2] != "-c" {
		t.Errorf("expected 'bash -c', got %v", last3)
	}
}

func TestContainerRunArgs_Mounts(t *testing.T) {
	cfg := ContainerRunConfig{
		Command:    "make",
		Image:      "yoe-ng/toolchain-musl:15-x86_64",
		ProjectDir: "/project",
		Mounts: []Mount{
			{Host: "/tmp/src", Container: "/build/src", ReadOnly: false},
			{Host: "/tmp/sysroot", Container: "/build/sysroot", ReadOnly: true},
		},
	}

	args, err := containerRunArgs(cfg)
	if err != nil {
		t.Fatalf("containerRunArgs: %v", err)
	}

	assertContains(t, args, "/tmp/src:/build/src")
	assertContains(t, args, "/tmp/sysroot:/build/sysroot:ro")
}

func TestContainerRunArgs_Env(t *testing.T) {
	cfg := ContainerRunConfig{
		Command:    "make",
		Image:      "yoe-ng/toolchain-musl:15-x86_64",
		ProjectDir: "/project",
		Env:        map[string]string{"PREFIX": "/usr", "NPROC": "4"},
	}

	args, err := containerRunArgs(cfg)
	if err != nil {
		t.Fatalf("containerRunArgs: %v", err)
	}

	assertContains(t, args, "-e")
	found := false
	for _, a := range args {
		if a == "PREFIX=/usr" || a == "NPROC=4" {
			found = true
		}
	}
	if !found {
		t.Error("env vars not found in args")
	}
}

func TestContainerRunArgs_Interactive(t *testing.T) {
	cfg := ContainerRunConfig{
		Command:     "qemu-system-x86_64",
		Image:       "yoe-ng/toolchain-musl:15-x86_64",
		ProjectDir:  "/project",
		Interactive: true,
	}

	args, err := containerRunArgs(cfg)
	if err != nil {
		t.Fatalf("containerRunArgs: %v", err)
	}

	assertContains(t, args, "-it")
}

func TestContainerRunArgs_NoUser(t *testing.T) {
	cfg := ContainerRunConfig{
		Command:    "losetup /dev/loop0 image.img",
		Image:      "yoe-ng/toolchain-musl:15-x86_64",
		ProjectDir: "/project",
		NoUser:     true,
	}

	args, err := containerRunArgs(cfg)
	if err != nil {
		t.Fatalf("containerRunArgs: %v", err)
	}

	for _, a := range args {
		if a == "--user" {
			t.Error("should not have --user when NoUser is true")
		}
	}
}

func assertContains(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Errorf("args %v does not contain %q", args, want)
}
