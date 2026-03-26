package starlark

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// LoadCommands discovers and evaluates commands/*.star files in the project.
// Returns the commands map and an engine for each command file (needed to
// retrieve the run() function later).
func LoadCommands(projectRoot string) (map[string]*Command, map[string]*Engine, error) {
	pattern := filepath.Join(projectRoot, "commands", "*.star")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("globbing %s: %w", pattern, err)
	}

	commands := make(map[string]*Command)
	engines := make(map[string]*Engine)

	for _, path := range matches {
		eng := NewEngine()
		if err := eng.ExecFile(path); err != nil {
			return nil, nil, fmt.Errorf("evaluating command %s: %w", path, err)
		}
		for name, cmd := range eng.Commands() {
			commands[name] = cmd
			engines[name] = eng
		}
	}

	return commands, engines, nil
}

// RunCommand executes a custom command's run() function with parsed arguments.
func RunCommand(eng *Engine, cmd *Command, args []string, projectRoot string) error {
	// Parse command-line args into a dict
	parsed, err := parseCommandArgs(cmd, args)
	if err != nil {
		return err
	}

	// Find the run() function in the command's globals
	runFn, ok := eng.Globals()["run"]
	if !ok {
		return fmt.Errorf("command %q has no run() function in %s", cmd.Name, cmd.SourceFile)
	}
	callable, ok := runFn.(starlark.Callable)
	if !ok {
		return fmt.Errorf("run in %s is not callable", cmd.SourceFile)
	}

	// Build the context object
	ctx := buildCommandContext(parsed, projectRoot)

	// Call run(ctx)
	thread := &starlark.Thread{Name: cmd.Name}
	_, err = starlark.Call(thread, callable, starlark.Tuple{ctx}, nil)
	return err
}

func parseCommandArgs(cmd *Command, args []string) (map[string]string, error) {
	parsed := make(map[string]string)

	// Set defaults
	for _, a := range cmd.Args {
		if a.Default != "" {
			parsed[cleanArgName(a.Name)] = a.Default
		}
		if a.IsBool {
			parsed[cleanArgName(a.Name)] = "false"
		}
	}

	// Parse positional and flag args
	positionalIdx := 0
	positionalArgs := make([]CommandArg, 0)
	for _, a := range cmd.Args {
		if !strings.HasPrefix(a.Name, "-") {
			positionalArgs = append(positionalArgs, a)
		}
	}

	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			key := strings.TrimPrefix(args[i], "--")
			// Find matching arg
			found := false
			for _, a := range cmd.Args {
				if cleanArgName(a.Name) == key {
					found = true
					if a.IsBool {
						parsed[key] = "true"
					} else if i+1 < len(args) {
						i++
						parsed[key] = args[i]
					} else {
						return nil, fmt.Errorf("flag --%s requires a value", key)
					}
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("unknown flag: %s", args[i])
			}
		} else {
			// Positional
			if positionalIdx < len(positionalArgs) {
				parsed[cleanArgName(positionalArgs[positionalIdx].Name)] = args[i]
				positionalIdx++
			} else {
				return nil, fmt.Errorf("unexpected argument: %s", args[i])
			}
		}
	}

	// Check required args
	for _, a := range cmd.Args {
		if a.Required {
			if _, ok := parsed[cleanArgName(a.Name)]; !ok {
				return nil, fmt.Errorf("required argument %q not provided", a.Name)
			}
		}
	}

	return parsed, nil
}

func cleanArgName(name string) string {
	return strings.TrimLeft(name, "-")
}

func buildCommandContext(args map[string]string, projectRoot string) *starlarkstruct.Struct {
	// Build args struct
	argsDict := make(starlark.StringDict, len(args))
	for k, v := range args {
		argsDict[k] = starlark.String(v)
	}
	argsStruct := starlarkstruct.FromStringDict(starlark.String("args"), argsDict)

	// Build context with args and helper functions
	ctxDict := starlark.StringDict{
		"args":         argsStruct,
		"project_root": starlark.String(projectRoot),
		"shell":        starlark.NewBuiltin("shell", ctxShell),
		"log":          starlark.NewBuiltin("log", ctxLog),
	}

	return starlarkstruct.FromStringDict(starlark.String("ctx"), ctxDict)
}

// ctxShell executes a shell command. Unlike recipe evaluation, command
// execution has full I/O access.
func ctxShell(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("shell() requires at least one argument")
	}

	var cmdArgs []string
	for _, a := range args {
		s, ok := a.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("shell() arguments must be strings")
		}
		cmdArgs = append(cmdArgs, string(s))
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdout = nil // TODO: wire to yoe's stdout
	cmd.Stderr = nil
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("shell %s: %s\n%s", cmdArgs[0], err, string(out))
	}

	return starlark.String(string(out)), nil
}

func ctxLog(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	var parts []string
	for _, a := range args {
		parts = append(parts, a.String())
	}
	fmt.Println(strings.Join(parts, " "))
	return starlark.None, nil
}
