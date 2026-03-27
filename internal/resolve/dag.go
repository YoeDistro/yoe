package resolve

import (
	"fmt"
	"sort"
	"strings"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// DAG represents the dependency graph of all recipes in a project.
type DAG struct {
	Nodes map[string]*Node
}

// Node represents a recipe in the dependency graph.
type Node struct {
	Recipe *yoestar.Recipe
	Deps   []string // build-time dependency names
	Rdeps  []string // reverse dependencies (computed)
}

// BuildDAG constructs a dependency graph from a loaded project.
func BuildDAG(proj *yoestar.Project) (*DAG, error) {
	dag := &DAG{Nodes: make(map[string]*Node)}

	// Add all recipes as nodes.
	// For image recipes, Packages are also dependencies (they must be built
	// before the image can be assembled).
	for name, recipe := range proj.Recipes {
		deps := recipe.Deps
		if recipe.Class == "image" {
			deps = append(append([]string{}, deps...), recipe.Packages...)
		}
		dag.Nodes[name] = &Node{
			Recipe: recipe,
			Deps:   deps,
		}
	}

	// Validate that all dependencies exist and compute reverse deps
	for name, node := range dag.Nodes {
		for _, dep := range node.Deps {
			target, ok := dag.Nodes[dep]
			if !ok {
				return nil, fmt.Errorf("recipe %q depends on %q, which does not exist", name, dep)
			}
			target.Rdeps = append(target.Rdeps, name)
		}
	}

	// Sort rdeps for deterministic output
	for _, node := range dag.Nodes {
		sort.Strings(node.Rdeps)
	}

	return dag, nil
}

// TopologicalSort returns recipes in build order (dependencies before dependents).
// Returns an error if the graph contains a cycle.
func (d *DAG) TopologicalSort() ([]string, error) {
	// Kahn's algorithm
	inDegree := make(map[string]int)
	for name := range d.Nodes {
		inDegree[name] = 0
	}
	for _, node := range d.Nodes {
		for _, dep := range node.Deps {
			inDegree[dep]++ // note: reversed — dep must come first
		}
	}

	// Actually we want: inDegree[x] = number of deps x has (not rdeps)
	// Kahn's: start with nodes that have no dependencies
	inDegree = make(map[string]int)
	for name, node := range d.Nodes {
		inDegree[name] = len(node.Deps)
	}

	// Queue starts with nodes that have no dependencies
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue) // deterministic order

	var order []string
	for len(queue) > 0 {
		// Pop first
		name := queue[0]
		queue = queue[1:]
		order = append(order, name)

		// For each node that depends on this one, decrement in-degree
		node := d.Nodes[name]
		for _, rdep := range node.Rdeps {
			inDegree[rdep]--
			if inDegree[rdep] == 0 {
				queue = append(queue, rdep)
				sort.Strings(queue) // keep deterministic
			}
		}
	}

	if len(order) != len(d.Nodes) {
		// Find the cycle for a useful error message
		var cycleNodes []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycleNodes = append(cycleNodes, name)
			}
		}
		sort.Strings(cycleNodes)
		return nil, fmt.Errorf("dependency cycle detected involving: %s", strings.Join(cycleNodes, ", "))
	}

	return order, nil
}

// DepsOf returns the transitive dependencies of a recipe (not including itself).
func (d *DAG) DepsOf(name string) ([]string, error) {
	if _, ok := d.Nodes[name]; !ok {
		return nil, fmt.Errorf("recipe %q not found", name)
	}

	visited := make(map[string]bool)
	var result []string

	var walk func(n string)
	walk = func(n string) {
		node := d.Nodes[n]
		for _, dep := range node.Deps {
			if !visited[dep] {
				visited[dep] = true
				result = append(result, dep)
				walk(dep)
			}
		}
	}

	walk(name)
	sort.Strings(result)
	return result, nil
}

// RdepsOf returns the transitive reverse dependencies (what depends on name).
func (d *DAG) RdepsOf(name string) ([]string, error) {
	if _, ok := d.Nodes[name]; !ok {
		return nil, fmt.Errorf("recipe %q not found", name)
	}

	visited := make(map[string]bool)
	var result []string

	var walk func(n string)
	walk = func(n string) {
		node := d.Nodes[n]
		for _, rdep := range node.Rdeps {
			if !visited[rdep] {
				visited[rdep] = true
				result = append(result, rdep)
				walk(rdep)
			}
		}
	}

	walk(name)
	sort.Strings(result)
	return result, nil
}
