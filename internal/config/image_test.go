package config

import (
	"testing"
)

func TestParseImageConfig(t *testing.T) {
	cfg, err := ParseImageConfig("../../testdata/valid-project/images/base.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Image.Name != "base" {
		t.Errorf("expected name %q, got %q", "base", cfg.Image.Name)
	}
	if cfg.Image.Description != "Minimal bootable system" {
		t.Errorf("expected description %q, got %q", "Minimal bootable system", cfg.Image.Description)
	}
	if len(cfg.Packages.Include) != 3 {
		t.Errorf("expected 3 packages, got %d", len(cfg.Packages.Include))
	}
	if cfg.Config.Hostname != "yoe" {
		t.Errorf("expected hostname %q, got %q", "yoe", cfg.Config.Hostname)
	}
	if cfg.Config.Timezone != "UTC" {
		t.Errorf("expected timezone %q, got %q", "UTC", cfg.Config.Timezone)
	}
	if cfg.Config.Locale != "en_US.UTF-8" {
		t.Errorf("expected locale %q, got %q", "en_US.UTF-8", cfg.Config.Locale)
	}
	if len(cfg.Services.Enable) != 3 {
		t.Errorf("expected 3 services, got %d", len(cfg.Services.Enable))
	}
}
