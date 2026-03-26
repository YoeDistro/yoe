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

// SetLayerRoot registers a named layer path for "@name//" module references.
func (e *Engine) SetLayerRoot(name, root string) {
	if e.layerRoots == nil {
		e.layerRoots = make(map[string]string)
	}
	e.layerRoots[name] = root
}

// makeLoadFunc returns a Starlark Load handler that resolves modules relative
// to fromFile and supports "//path", "@layer//path", and relative paths.
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

		globals, err := starlark.ExecFile(childThread, absPath, nil, predeclared)

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
// is inside a layer directory, returns that layer root; otherwise returns the
// project root.
func (e *Engine) rootForFile(file string) string {
	absFile, _ := filepath.Abs(file)
	for _, layerRoot := range e.layerRoots {
		absLayer, _ := filepath.Abs(layerRoot)
		if strings.HasPrefix(absFile, absLayer+string(filepath.Separator)) {
			return absLayer
		}
	}
	return e.projectRoot
}

// resolveLoadPath converts a module string to an absolute filesystem path.
//
// Supported forms:
//   - "//path"         -> projectRoot/path
//   - "@layer//path"   -> layerRoots[layer]/path
//   - "relative/path"  -> dir(fromFile)/relative/path
func (e *Engine) resolveLoadPath(fromFile, module string) (string, error) {
	switch {
	case strings.HasPrefix(module, "@"):
		// @layer//path
		idx := strings.Index(module, "//")
		if idx < 0 {
			return "", fmt.Errorf("invalid layer reference %q: expected @name//path", module)
		}
		layerName := module[1:idx]
		relPath := module[idx+2:]
		root, ok := e.layerRoots[layerName]
		if !ok {
			return "", fmt.Errorf("unknown layer %q in load(%q)", layerName, module)
		}
		return filepath.Join(root, relPath), nil

	case strings.HasPrefix(module, "//"):
		// Root-relative — resolve to the layer root if fromFile is inside a
		// layer, otherwise to the project root.
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
