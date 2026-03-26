package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type PartitionConfig struct {
	Disk      DiskConfig  `toml:"disk"`
	Partition []Partition `toml:"partition"`
}

type DiskConfig struct {
	Type string `toml:"type"`
}

type Partition struct {
	Label    string   `toml:"label"`
	Type     string   `toml:"type"`
	Size     string   `toml:"size"`
	Root     bool     `toml:"root"`
	Contents []string `toml:"contents"`
}

func ParsePartitionConfig(path string) (*PartitionConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading partition config: %w", err)
	}

	var cfg PartitionConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing partition config %s: %w", path, err)
	}

	if cfg.Disk.Type == "" {
		return nil, fmt.Errorf("partition config %s: disk.type is required", path)
	}
	if cfg.Disk.Type != "gpt" && cfg.Disk.Type != "mbr" {
		return nil, fmt.Errorf("partition config %s: disk.type must be \"gpt\" or \"mbr\"", path)
	}

	hasRoot := false
	for _, p := range cfg.Partition {
		if p.Root {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		return nil, fmt.Errorf("partition config %s: at least one partition must have root=true", path)
	}

	return &cfg, nil
}
