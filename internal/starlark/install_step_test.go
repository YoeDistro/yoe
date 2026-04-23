package starlark

import (
	"testing"

	"go.starlark.net/starlark"
)

func TestInstallFile_ReturnsValue(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	predeclared := starlark.StringDict{
		"install_file":     starlark.NewBuiltin("install_file", fnInstallFile),
		"install_template": starlark.NewBuiltin("install_template", fnInstallTemplate),
	}
	globals, err := starlark.ExecFile(thread, "t.star", `
f = install_file("rcS", "$DESTDIR/etc/init.d/rcS", mode = 0o755)
t = install_template("inittab.tmpl", "$DESTDIR/etc/inittab")
`, predeclared)
	if err != nil {
		t.Fatalf("ExecFile: %v", err)
	}
	f, ok := globals["f"].(*InstallStepValue)
	if !ok {
		t.Fatalf("f = %T, want *InstallStepValue", globals["f"])
	}
	if f.Kind != "file" || f.Src != "rcS" || f.Dest != "$DESTDIR/etc/init.d/rcS" || f.Mode != 0o755 {
		t.Errorf("f = %+v, unexpected fields", f)
	}
	tt, ok := globals["t"].(*InstallStepValue)
	if !ok {
		t.Fatalf("t = %T, want *InstallStepValue", globals["t"])
	}
	if tt.Kind != "template" || tt.Mode != 0o644 {
		t.Errorf("t = %+v, want default mode 0o644", tt)
	}
}

func TestInstallStepValue_HashStable(t *testing.T) {
	a := &InstallStepValue{Kind: "file", Src: "a", Dest: "/b", Mode: 0o644}
	b := &InstallStepValue{Kind: "file", Src: "a", Dest: "/b", Mode: 0o644}
	ha, _ := a.Hash()
	hb, _ := b.Hash()
	if ha != hb {
		t.Errorf("equal values hash differently: %d vs %d", ha, hb)
	}
	c := &InstallStepValue{Kind: "file", Src: "a", Dest: "/b", Mode: 0o755}
	hc, _ := c.Hash()
	if ha == hc {
		t.Error("different modes should produce different hashes")
	}
}
