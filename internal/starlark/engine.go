package starlark

import (
	"fmt"
	"sync"

	"go.starlark.net/starlark"
)

// Engine evaluates .star files and collects results.
type Engine struct {
	mu        sync.Mutex
	project   *Project
	machines  map[string]*Machine
	units     map[string]*Unit
	commands  map[string]*Command
	layerInfo *LayerInfo

	// globals stores the top-level bindings from the last ExecFile/ExecString,
	// used to retrieve the run() function for custom commands.
	globals starlark.StringDict

	// load() support
	projectRoot string
	layerRoots  map[string]string
	loadCache   *loadCache
}

func NewEngine() *Engine {
	return &Engine{
		machines: make(map[string]*Machine),
		units:    make(map[string]*Unit),
		commands: make(map[string]*Command),
	}
}

func (e *Engine) Project() *Project              { return e.project }
func (e *Engine) Machines() map[string]*Machine   { return e.machines }
func (e *Engine) Units() map[string]*Unit     { return e.units }
func (e *Engine) Commands() map[string]*Command   { return e.commands }
func (e *Engine) LayerInfo() *LayerInfo           { return e.layerInfo }
func (e *Engine) Globals() starlark.StringDict    { return e.globals }

// ExecString evaluates Starlark source code with built-in functions available.
func (e *Engine) ExecString(filename, src string) error {
	thread := &starlark.Thread{Name: filename}
	thread.Load = e.makeLoadFunc(filename)
	predeclared := e.builtins()

	globals, err := starlark.ExecFile(thread, filename, src, predeclared)
	if err != nil {
		return fmt.Errorf("evaluating %s: %w", filename, err)
	}
	e.globals = globals
	return nil
}

// ExecFile evaluates a .star file from disk.
func (e *Engine) ExecFile(path string) error {
	thread := &starlark.Thread{Name: path}
	thread.Load = e.makeLoadFunc(path)
	predeclared := e.builtins()

	globals, err := starlark.ExecFile(thread, path, nil, predeclared)
	if err != nil {
		return fmt.Errorf("evaluating %s: %w", path, err)
	}
	e.globals = globals
	return nil
}
