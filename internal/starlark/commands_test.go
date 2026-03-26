package starlark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvalCommand(t *testing.T) {
	src := `
command(
    name = "deploy",
    description = "Deploy to target",
    args = [
        arg("target", required=True, help="Target hostname"),
        arg("--image", default="base-image", help="Image to deploy"),
        arg("--reboot", type="bool", help="Reboot after deploy"),
    ],
)

def run(ctx):
    ctx.log("Deploying", ctx.args.image, "to", ctx.args.target)
`
	eng := NewEngine()
	if err := eng.ExecString("commands/deploy.star", src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}

	cmds := eng.Commands()
	cmd, ok := cmds["deploy"]
	if !ok {
		t.Fatal("command 'deploy' not found")
	}

	if cmd.Description != "Deploy to target" {
		t.Errorf("Description = %q, want %q", cmd.Description, "Deploy to target")
	}
	if len(cmd.Args) != 3 {
		t.Fatalf("Args has %d entries, want 3", len(cmd.Args))
	}
	if cmd.Args[0].Name != "target" {
		t.Errorf("Args[0].Name = %q, want %q", cmd.Args[0].Name, "target")
	}
	if !cmd.Args[0].Required {
		t.Error("Args[0].Required = false, want true")
	}
	if cmd.Args[1].Default != "base-image" {
		t.Errorf("Args[1].Default = %q, want %q", cmd.Args[1].Default, "base-image")
	}
	if !cmd.Args[2].IsBool {
		t.Error("Args[2].IsBool = false, want true")
	}

	// Verify run() function exists in globals
	if _, ok := eng.Globals()["run"]; !ok {
		t.Error("run() function not found in globals")
	}
}

func TestParseCommandArgs(t *testing.T) {
	cmd := &Command{
		Name: "deploy",
		Args: []CommandArg{
			{Name: "target", Required: true},
			{Name: "--image", Default: "base-image"},
			{Name: "--reboot", IsBool: true},
		},
	}

	parsed, err := parseCommandArgs(cmd, []string{"192.168.1.1", "--image", "prod", "--reboot"})
	if err != nil {
		t.Fatalf("parseCommandArgs: %v", err)
	}

	if parsed["target"] != "192.168.1.1" {
		t.Errorf("target = %q, want %q", parsed["target"], "192.168.1.1")
	}
	if parsed["image"] != "prod" {
		t.Errorf("image = %q, want %q", parsed["image"], "prod")
	}
	if parsed["reboot"] != "true" {
		t.Errorf("reboot = %q, want %q", parsed["reboot"], "true")
	}
}

func TestParseCommandArgs_Defaults(t *testing.T) {
	cmd := &Command{
		Name: "deploy",
		Args: []CommandArg{
			{Name: "target", Required: true},
			{Name: "--image", Default: "base-image"},
		},
	}

	parsed, err := parseCommandArgs(cmd, []string{"myhost"})
	if err != nil {
		t.Fatalf("parseCommandArgs: %v", err)
	}

	if parsed["image"] != "base-image" {
		t.Errorf("image = %q, want default %q", parsed["image"], "base-image")
	}
}

func TestParseCommandArgs_MissingRequired(t *testing.T) {
	cmd := &Command{
		Name: "deploy",
		Args: []CommandArg{
			{Name: "target", Required: true},
		},
	}

	_, err := parseCommandArgs(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for missing required arg, got nil")
	}
}

func TestLoadCommands(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, "commands")
	os.MkdirAll(cmdDir, 0755)

	os.WriteFile(filepath.Join(cmdDir, "hello.star"), []byte(`
command(
    name = "hello",
    description = "Say hello",
    args = [arg("name", default="world")],
)

def run(ctx):
    ctx.log("Hello", ctx.args.name)
`), 0644)

	cmds, engines, err := LoadCommands(dir)
	if err != nil {
		t.Fatalf("LoadCommands: %v", err)
	}

	if len(cmds) != 1 {
		t.Errorf("got %d commands, want 1", len(cmds))
	}
	if _, ok := cmds["hello"]; !ok {
		t.Error("command 'hello' not found")
	}
	if _, ok := engines["hello"]; !ok {
		t.Error("engine for 'hello' not found")
	}
}

func TestRunCommand(t *testing.T) {
	src := `
command(
    name = "greet",
    description = "Greet someone",
    args = [arg("name", default="world")],
)

def run(ctx):
    ctx.log("Hello", ctx.args.name)
`
	eng := NewEngine()
	if err := eng.ExecString("commands/greet.star", src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}

	cmd := eng.Commands()["greet"]
	err := RunCommand(eng, cmd, []string{"Yoe"}, t.TempDir())
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
}
