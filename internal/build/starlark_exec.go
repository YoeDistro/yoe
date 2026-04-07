package build

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Execer abstracts command execution for testability.
type Execer interface {
	Run(ctx context.Context, cfg *SandboxConfig, command string, privileged bool) (ExecResult, error)
	RunHost(ctx context.Context, command string, dir string) (ExecResult, error)
}

// ExecResult holds the outcome of a sandboxed command execution.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// RealExecer executes commands via RunInSandbox.
type RealExecer struct{}

func (RealExecer) Run(ctx context.Context, cfg *SandboxConfig, command string, privileged bool) (ExecResult, error) {
	cfg.Ctx = ctx

	// Capture stdout/stderr into buffers while still writing to the log.
	var stdoutBuf, stderrBuf bytes.Buffer
	origStdout, origStderr := cfg.Stdout, cfg.Stderr
	if origStdout != nil {
		cfg.Stdout = io.MultiWriter(origStdout, &stdoutBuf)
	} else {
		cfg.Stdout = &stdoutBuf
	}
	if origStderr != nil {
		cfg.Stderr = io.MultiWriter(origStderr, &stderrBuf)
	} else {
		cfg.Stderr = &stderrBuf
	}

	var err error
	if privileged {
		// Run directly in container without bwrap and as root
		// (for losetup, mount, extlinux, etc.)
		cfg.NoUser = true
		err = RunSimple(cfg, command)
		cfg.NoUser = false
	} else {
		err = RunInSandbox(cfg, command)
	}

	// Restore original writers
	cfg.Stdout, cfg.Stderr = origStdout, origStderr

	if err != nil {
		return ExecResult{ExitCode: 1, Stdout: stdoutBuf.String(), Stderr: stderrBuf.String()}, err
	}
	return ExecResult{ExitCode: 0, Stdout: stdoutBuf.String(), Stderr: stderrBuf.String()}, nil
}

func (RealExecer) RunHost(ctx context.Context, command string, dir string) (ExecResult, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
	}
	return ExecResult{
		ExitCode: exitCode,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
	}, err
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
	// Store the real run() so the global placeholder can delegate.
	t.SetLocal("yoe.run", starlark.NewBuiltin("run", fnRun))
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
	privileged := false
	host := false
	for _, kv := range kwargs {
		key := string(kv[0].(starlark.String))
		if key == "check" {
			if b, ok := kv[1].(starlark.Bool); ok {
				check = bool(b)
			}
		}
		if key == "privileged" {
			if b, ok := kv[1].(starlark.Bool); ok {
				privileged = bool(b)
			}
		}
		if key == "host" {
			if b, ok := kv[1].(starlark.Bool); ok {
				host = bool(b)
			}
		}
	}

	if host {
		ctx := thread.Local(contextKey).(context.Context)
		execer := thread.Local(execerKey).(Execer)
		cfg := thread.Local(sandboxKey).(*SandboxConfig)
		result, err := execer.RunHost(ctx, string(command), cfg.HostDir)

		resultStruct := starlarkstruct.FromStringDict(starlark.String("result"), starlark.StringDict{
			"exit_code": starlark.MakeInt(result.ExitCode),
			"stdout":    starlark.String(result.Stdout),
			"stderr":    starlark.String(result.Stderr),
		})

		if err != nil && check {
			return nil, fmt.Errorf("run(%q, host=True) failed: exit code %d\n%s",
				string(command), result.ExitCode, result.Stderr)
		}

		return resultStruct, nil
	}

	cfg := thread.Local(sandboxKey).(*SandboxConfig)
	execer := thread.Local(execerKey).(Execer)
	ctx := thread.Local(contextKey).(context.Context)

	result, err := execer.Run(ctx, cfg, string(command), privileged)

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
