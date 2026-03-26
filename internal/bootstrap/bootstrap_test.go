package bootstrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

func TestStatus(t *testing.T) {
	projectDir := t.TempDir()
	os.MkdirAll(filepath.Join(projectDir, "build", "repo"), 0755)

	proj := &yoestar.Project{
		Name: "test",
		Recipes: map[string]*yoestar.Recipe{
			"glibc":   {Name: "glibc", Version: "2.39"},
			"gcc":     {Name: "gcc", Version: "14.1"},
			"busybox": {Name: "busybox", Version: "1.36"},
		},
	}

	var buf bytes.Buffer
	if err := Status(proj, projectDir, &buf); err != nil {
		t.Fatalf("Status: %v", err)
	}

	output := buf.String()

	// Should list all bootstrap recipes
	if !strings.Contains(output, "glibc") {
		t.Error("should list glibc")
	}
	if !strings.Contains(output, "gcc") {
		t.Error("should list gcc")
	}

	// Recipes that exist should say "recipe found"
	if !strings.Contains(output, "recipe found") {
		t.Error("should show 'recipe found' for existing recipes")
	}

	// Missing recipes should say "missing"
	if !strings.Contains(output, "missing") {
		t.Error("should show 'missing' for missing recipes")
	}
}

func TestStage0_MissingRecipes(t *testing.T) {
	proj := &yoestar.Project{
		Name:    "test",
		Recipes: map[string]*yoestar.Recipe{},
	}

	var buf bytes.Buffer
	err := Stage0(proj, t.TempDir(), &buf)
	if err == nil {
		t.Fatal("expected error for missing bootstrap recipes")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention missing recipes: %v", err)
	}
}

func TestStage0Commands(t *testing.T) {
	recipe := &yoestar.Recipe{
		Name:  "test",
		Class: "autotools",
		ConfigureArgs: []string{"--with-glibc"},
	}

	cmds := stage0Commands(recipe)
	if len(cmds) != 3 {
		t.Errorf("expected 3 commands for autotools, got %d", len(cmds))
	}
	if len(cmds) > 0 && !strings.Contains(cmds[0], "--with-glibc") {
		t.Errorf("configure should include args: %s", cmds[0])
	}
}

func TestStage0Commands_ExplicitBuild(t *testing.T) {
	recipe := &yoestar.Recipe{
		Name:  "test",
		Build: []string{"make all", "make install"},
	}

	cmds := stage0Commands(recipe)
	if len(cmds) != 2 {
		t.Errorf("expected 2 explicit commands, got %d", len(cmds))
	}
}

func TestVerifyStage0_Missing(t *testing.T) {
	repoDir := t.TempDir()
	err := verifyStage0(repoDir)
	if err == nil {
		t.Fatal("expected error for empty repo")
	}
}
