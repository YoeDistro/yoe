package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type ImageConfig struct {
	Image    ImageInfo     `toml:"image"`
	Packages ImagePackages `toml:"packages"`
	Config   ImageSettings `toml:"config"`
	Services ImageServices `toml:"services"`
}

type ImageInfo struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

type ImagePackages struct {
	Include []string `toml:"include"`
}

type ImageSettings struct {
	Hostname string `toml:"hostname"`
	Timezone string `toml:"timezone"`
	Locale   string `toml:"locale"`
}

type ImageServices struct {
	Enable []string `toml:"enable"`
}

func ParseImageConfig(path string) (*ImageConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading image config: %w", err)
	}

	var cfg ImageConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing image config %s: %w", path, err)
	}

	if cfg.Image.Name == "" {
		return nil, fmt.Errorf("image config %s: image.name is required", path)
	}

	return &cfg, nil
}
