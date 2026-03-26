package config

import (
	"path/filepath"
	"testing"
)

func TestParseRecipeConfig_SystemPackage(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-project", "recipes", "openssh.toml")
	recipe, err := ParseRecipeConfig(path)
	if err != nil {
		t.Fatalf("ParseRecipeConfig(%q): %v", path, err)
	}

	if recipe.Recipe.Name != "openssh" {
		t.Errorf("Name = %q, want %q", recipe.Recipe.Name, "openssh")
	}
	if recipe.Recipe.Version != "9.6p1" {
		t.Errorf("Version = %q, want %q", recipe.Recipe.Version, "9.6p1")
	}
	if recipe.Recipe.Description != "OpenSSH client and server" {
		t.Errorf("Description = %q, want %q", recipe.Recipe.Description, "OpenSSH client and server")
	}
	if recipe.Recipe.License != "BSD" {
		t.Errorf("License = %q, want %q", recipe.Recipe.License, "BSD")
	}
	if recipe.Recipe.Language != "" {
		t.Errorf("Language = %q, want empty", recipe.Recipe.Language)
	}

	if recipe.Source.URL != "https://cdn.openbsd.org/pub/OpenBSD/OpenSSH/portable/openssh-9.6p1.tar.gz" {
		t.Errorf("Source.URL = %q, want correct URL", recipe.Source.URL)
	}
	if recipe.Source.SHA256 != "aaaa1111bbbb2222cccc3333dddd4444eeee5555ffff6666" {
		t.Errorf("Source.SHA256 = %q, want correct hash", recipe.Source.SHA256)
	}

	if len(recipe.Depends.Build) != 2 || recipe.Depends.Build[0] != "zlib" || recipe.Depends.Build[1] != "openssl" {
		t.Errorf("Depends.Build = %v, want [zlib openssl]", recipe.Depends.Build)
	}
	if len(recipe.Depends.Runtime) != 2 || recipe.Depends.Runtime[0] != "zlib" || recipe.Depends.Runtime[1] != "openssl" {
		t.Errorf("Depends.Runtime = %v, want [zlib openssl]", recipe.Depends.Runtime)
	}

	if len(recipe.Build.Steps) != 3 {
		t.Fatalf("Build.Steps length = %d, want 3", len(recipe.Build.Steps))
	}
	if recipe.Build.Steps[0] != "./configure --prefix=$PREFIX --sysconfdir=/etc/ssh" {
		t.Errorf("Build.Steps[0] = %q, want configure command", recipe.Build.Steps[0])
	}
	if recipe.Build.Command != "" {
		t.Errorf("Build.Command = %q, want empty", recipe.Build.Command)
	}

	if len(recipe.Package.Units) != 1 || recipe.Package.Units[0] != "sshd.service" {
		t.Errorf("Package.Units = %v, want [sshd.service]", recipe.Package.Units)
	}
	if len(recipe.Package.Conffiles) != 1 || recipe.Package.Conffiles[0] != "/etc/ssh/sshd_config" {
		t.Errorf("Package.Conffiles = %v, want [/etc/ssh/sshd_config]", recipe.Package.Conffiles)
	}
}

func TestParseRecipeConfig_AppPackage(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-project", "recipes", "myapp.toml")
	recipe, err := ParseRecipeConfig(path)
	if err != nil {
		t.Fatalf("ParseRecipeConfig(%q): %v", path, err)
	}

	if recipe.Recipe.Language != "go" {
		t.Errorf("Language = %q, want %q", recipe.Recipe.Language, "go")
	}
	if recipe.Source.Repo != "https://github.com/example/myapp.git" {
		t.Errorf("Source.Repo = %q, want correct repo URL", recipe.Source.Repo)
	}
	if recipe.Source.Tag != "v1.2.3" {
		t.Errorf("Source.Tag = %q, want %q", recipe.Source.Tag, "v1.2.3")
	}
	if recipe.Build.Command != "go build -o $DESTDIR/usr/bin/myapp ./cmd/myapp" {
		t.Errorf("Build.Command = %q, want go build command", recipe.Build.Command)
	}
	if len(recipe.Build.Steps) != 0 {
		t.Errorf("Build.Steps = %v, want empty", recipe.Build.Steps)
	}
}

func TestParseRecipeConfig_NoBuildMethod(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "invalid-project", "no-build-recipe.toml")
	_, err := ParseRecipeConfig(path)
	if err == nil {
		t.Fatal("expected error for recipe with no build method, got nil")
	}
}
