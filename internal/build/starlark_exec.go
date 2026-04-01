package build

import (
	"context"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Execer abstracts command execution for testability.
type Execer interface {
	Run(ctx context.Context, cfg *SandboxConfig, command string) (ExecResult, error)
}

// ExecResult holds the outcome of a sandboxed command execution.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// RealExecer executes commands via RunInSandbox.
type RealExecer struct{}

func (RealExecer) Run(ctx context.Context, cfg *SandboxConfig, command string) (ExecResult, error) {
	cfg.Ctx = ctx
	err := RunInSandbox(cfg, command)
	if err != nil {
		return ExecResult{ExitCode: 1, Stderr: err.Error()}, err
	}
	return ExecResult{ExitCode: 0}, nil
}

// Thread-local keys for build-time Starlark threads.
const sandboxKey = "yoe.sandbox"
const execerKey = "yoe.execer"
const contextKey = "yoe.context"

// NewBuildThread creates a Starlark thread wired up for build-time execution.
// The thread carries a sandbox config, an Execer, and a context in thread-local storage.
func NewBuildThread(ctx context.Context, cfg *SandboxConfig, execer Execer) *starlark.Thread {
	t := &starlark.Thread{Name: "build"}
	t.SetLocal(sandboxKey, cfg)
	t.SetLocal(execerKey, execer)
	t.SetLocal(contextKey, ctx)
	return t
}

// fnRun implements the run() Starlark builtin for build-time command execution.
//
//	run(command, check=True) -> struct(exit_code, stdout, stderr)
func fnRun(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var command starlark.String
	if err := starlark.UnpackPositionalArgs("run", args, nil, 1, &command); err != nil {
		return nil, err
	}

	check := true
	for _, kv := range kwargs {
		if string(kv[0].(starlark.String)) == "check" {
			if b, ok := kv[1].(starlark.Bool); ok {
				check = bool(b)
			}
		}
	}

	cfg := thread.Local(sandboxKey).(*SandboxConfig)
	execer := thread.Local(execerKey).(Execer)
	ctx := thread.Local(contextKey).(context.Context)

	result, err := execer.Run(ctx, cfg, string(command))

	resultStruct := starlarkstruct.FromStringDict(starlark.String("result"), starlark.StringDict{
		"exit_code": starlark.MakeInt(result.ExitCode),
		"stdout":    starlark.String(result.Stdout),
		"stderr":    starlark.String(result.Stderr),
	})

	if err != nil && check {
		return nil, fmt.Errorf("run(%q) failed: exit code %d\n%s",
			string(command), result.ExitCode, result.Stderr)
	}

	return resultStruct, nil
}

// BuildPredeclared returns the predeclared names available in build-time
// Starlark threads. Currently provides only run().
func BuildPredeclared() starlark.StringDict {
	return starlark.StringDict{
		"run": starlark.NewBuiltin("run", fnRun),
	}
}
