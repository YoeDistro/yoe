package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type DistroConfig struct {
	Distro     DistroInfo          `toml:"distro"`
	Defaults   DistroDefaults      `toml:"defaults"`
	Repository RepoConfig          `toml:"repository"`
	Cache      CacheConfig         `toml:"cache"`
	Sources    SourcesConfig       `toml:"sources"`
	Layers     map[string]LayerRef `toml:"layers"`
}

type DistroInfo struct {
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Description string `toml:"description"`
}

type DistroDefaults struct {
	Machine string `toml:"machine"`
	Image   string `toml:"image"`
}

type RepoConfig struct {
	Path   string `toml:"path"`
	Remote string `toml:"remote"`
}

type CacheConfig struct {
	Path string `toml:"path"`
}

type SourcesConfig struct {
	GoProxy       string `toml:"go-proxy"`
	CargoRegistry string `toml:"cargo-registry"`
	NpmRegistry   string `toml:"npm-registry"`
}

type LayerRef struct {
	URL string `toml:"url"`
	Ref string `toml:"ref"`
}

func ParseDistroConfig(path string) (*DistroConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading distro config: %w", err)
	}

	var cfg DistroConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing distro config %s: %w", path, err)
	}

	if cfg.Distro.Name == "" {
		return nil, fmt.Errorf("distro config %s: distro.name is required", path)
	}

	return &cfg, nil
}
