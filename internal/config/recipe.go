package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type RecipeConfig struct {
	Recipe  RecipeInfo    `toml:"recipe"`
	Source  SourceConfig  `toml:"source"`
	Depends DependsConfig `toml:"depends"`
	Build   BuildConfig   `toml:"build"`
	Package PackageConfig `toml:"package"`
}

type RecipeInfo struct {
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Description string `toml:"description"`
	License     string `toml:"license"`
	Language    string `toml:"language"`
}

type SourceConfig struct {
	URL    string `toml:"url"`
	SHA256 string `toml:"sha256"`
	Repo   string `toml:"repo"`
	Tag    string `toml:"tag"`
	Branch string `toml:"branch"`
}

type DependsConfig struct {
	Build   []string `toml:"build"`
	Runtime []string `toml:"runtime"`
}

type BuildConfig struct {
	Steps   []string `toml:"steps"`
	Command string   `toml:"command"`
}

type PackageConfig struct {
	Units       []string          `toml:"units"`
	Conffiles   []string          `toml:"conffiles"`
	Environment map[string]string `toml:"environment"`
}

func ParseRecipeConfig(path string) (*RecipeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading recipe config: %w", err)
	}

	var cfg RecipeConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing recipe config %s: %w", path, err)
	}

	if cfg.Recipe.Name == "" {
		return nil, fmt.Errorf("recipe config %s: recipe.name is required", path)
	}
	if cfg.Recipe.Version == "" {
		return nil, fmt.Errorf("recipe config %s: recipe.version is required", path)
	}
	if len(cfg.Build.Steps) == 0 && cfg.Build.Command == "" {
		return nil, fmt.Errorf("recipe config %s: build.steps or build.command is required", path)
	}

	return &cfg, nil
}
