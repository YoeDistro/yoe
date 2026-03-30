package resolve

import (
	"testing"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

func TestUnitHash_Deterministic(t *testing.T) {
	unit := &yoestar.Unit{
		Name:    "openssh",
		Version: "9.6p1",
		Class:   "package",
		Source:  "https://example.com/openssh.tar.gz",
		SHA256:  "abc123",
		Deps:    []string{"zlib"},
		Build:   []string{"make"},
	}

	h1 := UnitHash(unit, "arm64", map[string]string{"zlib": "deadbeef"})
	h2 := UnitHash(unit, "arm64", map[string]string{"zlib": "deadbeef"})

	if h1 != h2 {
		t.Errorf("hash not deterministic: %s != %s", h1, h2)
	}
	if len(h1) != 64 { // sha256 hex
		t.Errorf("hash length = %d, want 64", len(h1))
	}
}

func TestUnitHash_ChangesOnInput(t *testing.T) {
	unit := &yoestar.Unit{
		Name:    "openssh",
		Version: "9.6p1",
		Class:   "package",
		Source:  "https://example.com/openssh.tar.gz",
		Deps:    []string{"zlib"},
		Build:   []string{"make"},
	}

	h1 := UnitHash(unit, "arm64", map[string]string{"zlib": "aaa"})

	// Change dep hash
	h2 := UnitHash(unit, "arm64", map[string]string{"zlib": "bbb"})
	if h1 == h2 {
		t.Error("hash should change when dependency hash changes")
	}

	// Change arch
	h3 := UnitHash(unit, "x86_64", map[string]string{"zlib": "aaa"})
	if h1 == h3 {
		t.Error("hash should change when arch changes")
	}

	// Change version
	unit2 := *unit
	unit2.Version = "9.7p1"
	h4 := UnitHash(&unit2, "arm64", map[string]string{"zlib": "aaa"})
	if h1 == h4 {
		t.Error("hash should change when version changes")
	}
}

func TestComputeAllHashes(t *testing.T) {
	proj := makeProject(map[string]*yoestar.Unit{
		"zlib":    {Name: "zlib", Version: "1.3", Class: "unit", Deps: nil, Build: []string{"make"}},
		"openssl": {Name: "openssl", Version: "3.0", Class: "unit", Deps: []string{"zlib"}, Build: []string{"make"}},
		"openssh": {Name: "openssh", Version: "9.6", Class: "unit", Deps: []string{"zlib", "openssl"}, Build: []string{"make"}},
	})

	dag, err := BuildDAG(proj)
	if err != nil {
		t.Fatalf("BuildDAG: %v", err)
	}

	hashes, err := ComputeAllHashes(dag, "arm64")
	if err != nil {
		t.Fatalf("ComputeAllHashes: %v", err)
	}

	if len(hashes) != 3 {
		t.Errorf("got %d hashes, want 3", len(hashes))
	}

	// All hashes should be different
	if hashes["zlib"] == hashes["openssl"] {
		t.Error("zlib and openssl should have different hashes")
	}
	if hashes["openssl"] == hashes["openssh"] {
		t.Error("openssl and openssh should have different hashes")
	}

	// openssh hash includes openssl hash which includes zlib hash
	// Changing zlib should cascade
	proj.Units["zlib"].Version = "1.4"
	dag2, _ := BuildDAG(proj)
	hashes2, _ := ComputeAllHashes(dag2, "arm64")

	if hashes["zlib"] == hashes2["zlib"] {
		t.Error("zlib hash should change after version bump")
	}
	if hashes["openssh"] == hashes2["openssh"] {
		t.Error("openssh hash should change when transitive dep changes")
	}
}
