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
	recipes   map[string]*Recipe
	layerInfo *LayerInfo
}

func NewEngine() *Engine {
	return &Engine{
		machines: make(map[string]*Machine),
		recipes:  make(map[string]*Recipe),
	}
}

func (e *Engine) Project() *Project            { return e.project }
func (e *Engine) Machines() map[string]*Machine { return e.machines }
func (e *Engine) Recipes() map[string]*Recipe   { return e.recipes }
func (e *Engine) LayerInfo() *LayerInfo         { return e.layerInfo }

// ExecString evaluates Starlark source code with built-in functions available.
func (e *Engine) ExecString(filename, src string) error {
	thread := &starlark.Thread{Name: filename}
	predeclared := e.builtins()

	_, err := starlark.ExecFile(thread, filename, src, predeclared)
	if err != nil {
		return fmt.Errorf("evaluating %s: %w", filename, err)
	}
	return nil
}

// ExecFile evaluates a .star file from disk.
func (e *Engine) ExecFile(path string) error {
	thread := &starlark.Thread{Name: path}
	predeclared := e.builtins()

	_, err := starlark.ExecFile(thread, path, nil, predeclared)
	if err != nil {
		return fmt.Errorf("evaluating %s: %w", path, err)
	}
	return nil
}
