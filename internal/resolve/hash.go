package resolve

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// UnitHash computes the content-addressed cache key for a unit.
// The hash includes:
//   - Unit fields (name, version, class, source, sha256, deps, build steps, etc.)
//   - Machine architecture and build flags
//   - Dependency hashes (transitive, via depHashes map)
//
// This ensures any change to a unit, its source, or any of its dependencies
// produces a new hash and triggers a rebuild.
func UnitHash(unit *yoestar.Unit, arch string, depHashes map[string]string) string {
	h := sha256.New()

	// Unit identity
	fmt.Fprintf(h, "name:%s\n", unit.Name)
	fmt.Fprintf(h, "version:%s\n", unit.Version)
	fmt.Fprintf(h, "class:%s\n", unit.Class)
	fmt.Fprintf(h, "arch:%s\n", arch)

	// Source
	fmt.Fprintf(h, "source:%s\n", unit.Source)
	fmt.Fprintf(h, "sha256:%s\n", unit.SHA256)
	fmt.Fprintf(h, "tag:%s\n", unit.Tag)
	fmt.Fprintf(h, "branch:%s\n", unit.Branch)
	fmt.Fprintf(h, "patches:%s\n", strings.Join(unit.Patches, "|"))

	// Build configuration
	fmt.Fprintf(h, "build:%s\n", strings.Join(unit.Build, "|"))
	fmt.Fprintf(h, "configure_args:%s\n", strings.Join(unit.ConfigureArgs, "|"))
	fmt.Fprintf(h, "go_package:%s\n", unit.GoPackage)

	// Dependencies — include their hashes for transitivity
	deps := make([]string, len(unit.Deps))
	copy(deps, unit.Deps)
	sort.Strings(deps)
	for _, dep := range deps {
		if dh, ok := depHashes[dep]; ok {
			fmt.Fprintf(h, "dep:%s:%s\n", dep, dh)
		}
	}

	// Image-specific fields
	if unit.Class == "image" {
		pkgs := make([]string, len(unit.Artifacts))
		copy(pkgs, unit.Artifacts)
		sort.Strings(pkgs)
		fmt.Fprintf(h, "packages:%s\n", strings.Join(pkgs, ","))
		fmt.Fprintf(h, "hostname:%s\n", unit.Hostname)
		fmt.Fprintf(h, "timezone:%s\n", unit.Timezone)
		fmt.Fprintf(h, "locale:%s\n", unit.Locale)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// ComputeAllHashes computes hashes for all units in build order.
// Returns a map of unit name -> hash.
func ComputeAllHashes(dag *DAG, arch, machine string) (map[string]string, error) {
	order, err := dag.TopologicalSort()
	if err != nil {
		return nil, err
	}

	hashes := make(map[string]string, len(order))
	for _, name := range order {
		node := dag.Nodes[name]
		unitArch := arch
		// Machine-scoped units include the machine name in the hash
		// so the same unit built for different machines caches separately.
		if node.Unit.Scope == "machine" {
			unitArch = arch + ":" + machine
		}
		hashes[name] = UnitHash(node.Unit, unitArch, hashes)
	}

	return hashes, nil
}
