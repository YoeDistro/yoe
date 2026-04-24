package starlark

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"go.starlark.net/starlark"
)

// loadResult caches the outcome of loading a single module.
type loadResult struct {
	globals starlark.StringDict
	err     error
}

// loadCache prevents re-evaluating the same .star module.
type loadCache struct {
	mu      sync.Mutex
	entries map[string]*loadResult
}

func newLoadCache() *loadCache {
	return &loadCache{entries: make(map[string]*loadResult)}
}

// SetProjectRoot stores the root path used to resolve "//" module references.
func (e *Engine) SetProjectRoot(root string) {
	e.projectRoot = root
}

// SetModuleRoot registers a named module path for "@name//" load references.
func (e *Engine) SetModuleRoot(name, root string) {
	if e.moduleRoots == nil {
		e.moduleRoots = make(map[string]string)
	}
	e.moduleRoots[name] = root
}

// makeLoadFunc returns a Starlark Load handler that resolves modules relative
// to fromFile and supports "//path", "@module//path", and relative paths.
func (e *Engine) makeLoadFunc(fromFile string) func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
	if e.loadCache == nil {
		e.loadCache = newLoadCache()
	}

	return func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		absPath, err := e.resolveLoadPath(fromFile, module)
		if err != nil {
			return nil, err
		}

		// Check cache
		e.loadCache.mu.Lock()
		if result, ok := e.loadCache.entries[absPath]; ok {
			e.loadCache.mu.Unlock()
			return result.globals, result.err
		}
		// Reserve the slot to prevent concurrent duplicate loads
		e.loadCache.entries[absPath] = nil
		e.loadCache.mu.Unlock()

		// Execute the module with builtins available
		childThread := &starlark.Thread{Name: absPath}
		childThread.Load = e.makeLoadFunc(absPath)
		predeclared := e.builtins()

		globals, err := starlark.ExecFileOptions(fileOpts, childThread, absPath, nil, predeclared)

		result := &loadResult{globals: globals, err: err}
		e.loadCache.mu.Lock()
		e.loadCache.entries[absPath] = result
		e.loadCache.mu.Unlock()

		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", module, err)
		}
		return globals, nil
	}
}

// rootForFile returns the appropriate root directory for a file — if the file
// is inside a module directory, returns that module root; otherwise returns the
// project root.
func (e *Engine) rootForFile(file string) string {
	absFile, _ := filepath.Abs(file)
	for _, moduleRoot := range e.moduleRoots {
		absModule, _ := filepath.Abs(moduleRoot)
		if strings.HasPrefix(absFile, absModule+string(filepath.Separator)) {
			return absModule
		}
	}
	return e.projectRoot
}

// resolveLoadPath converts a module string to an absolute filesystem path.
//
// Supported forms:
//   - "//path"         -> projectRoot/path
//   - "@module//path"  -> moduleRoots[module]/path
//   - "relative/path"  -> dir(fromFile)/relative/path
func (e *Engine) resolveLoadPath(fromFile, module string) (string, error) {
	switch {
	case strings.HasPrefix(module, "@"):
		// @module//path
		idx := strings.Index(module, "//")
		if idx < 0 {
			return "", fmt.Errorf("invalid module reference %q: expected @name//path", module)
		}
		moduleName := module[1:idx]
		relPath := module[idx+2:]
		root, ok := e.moduleRoots[moduleName]
		if !ok {
			return "", fmt.Errorf("unknown module %q in load(%q)", moduleName, module)
		}
		return filepath.Join(root, relPath), nil

	case strings.HasPrefix(module, "//"):
		// Root-relative — resolve to the module root if fromFile is inside a
		// module, otherwise to the project root.
		root := e.rootForFile(fromFile)
		if root == "" {
			return "", fmt.Errorf("cannot resolve %q: no root for %s", module, fromFile)
		}
		return filepath.Join(root, module[2:]), nil

	default:
		// Relative to the loading file's directory
		dir := filepath.Dir(fromFile)
		return filepath.Join(dir, module), nil
	}
}
