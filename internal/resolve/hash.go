package resolve

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// RecipeHash computes the content-addressed cache key for a recipe.
// The hash includes:
//   - Recipe fields (name, version, class, source, sha256, deps, build steps, etc.)
//   - Machine architecture and build flags
//   - Dependency hashes (transitive, via depHashes map)
//
// This ensures any change to a recipe, its source, or any of its dependencies
// produces a new hash and triggers a rebuild.
func RecipeHash(recipe *yoestar.Recipe, arch string, depHashes map[string]string) string {
	h := sha256.New()

	// Recipe identity
	fmt.Fprintf(h, "name:%s\n", recipe.Name)
	fmt.Fprintf(h, "version:%s\n", recipe.Version)
	fmt.Fprintf(h, "class:%s\n", recipe.Class)
	fmt.Fprintf(h, "arch:%s\n", arch)

	// Source
	fmt.Fprintf(h, "source:%s\n", recipe.Source)
	fmt.Fprintf(h, "sha256:%s\n", recipe.SHA256)
	fmt.Fprintf(h, "tag:%s\n", recipe.Tag)
	fmt.Fprintf(h, "branch:%s\n", recipe.Branch)
	fmt.Fprintf(h, "patches:%s\n", strings.Join(recipe.Patches, "|"))

	// Build configuration
	fmt.Fprintf(h, "build:%s\n", strings.Join(recipe.Build, "|"))
	fmt.Fprintf(h, "configure_args:%s\n", strings.Join(recipe.ConfigureArgs, "|"))
	fmt.Fprintf(h, "go_package:%s\n", recipe.GoPackage)

	// Dependencies — include their hashes for transitivity
	deps := make([]string, len(recipe.Deps))
	copy(deps, recipe.Deps)
	sort.Strings(deps)
	for _, dep := range deps {
		if dh, ok := depHashes[dep]; ok {
			fmt.Fprintf(h, "dep:%s:%s\n", dep, dh)
		}
	}

	// Image-specific fields
	if recipe.Class == "image" {
		pkgs := make([]string, len(recipe.Packages))
		copy(pkgs, recipe.Packages)
		sort.Strings(pkgs)
		fmt.Fprintf(h, "packages:%s\n", strings.Join(pkgs, ","))
		fmt.Fprintf(h, "hostname:%s\n", recipe.Hostname)
		fmt.Fprintf(h, "timezone:%s\n", recipe.Timezone)
		fmt.Fprintf(h, "locale:%s\n", recipe.Locale)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// ComputeAllHashes computes hashes for all recipes in build order.
// Returns a map of recipe name -> hash.
func ComputeAllHashes(dag *DAG, arch string) (map[string]string, error) {
	order, err := dag.TopologicalSort()
	if err != nil {
		return nil, err
	}

	hashes := make(map[string]string, len(order))
	for _, name := range order {
		node := dag.Nodes[name]
		hashes[name] = RecipeHash(node.Recipe, arch, hashes)
	}

	return hashes, nil
}
