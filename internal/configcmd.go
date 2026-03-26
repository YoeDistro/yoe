package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/YoeDistro/yoe-ng/internal/config"
)

func RunConfigShow(dir string, w io.Writer) error {
	distro, err := config.ParseDistroConfig(filepath.Join(dir, "distro.toml"))
	if err != nil {
		return err
	}
	return toml.NewEncoder(w).Encode(distro)
}

func RunConfigSet(dir, key, value string) error {
	path := filepath.Join(dir, "distro.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading distro.toml: %w", err)
	}

	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing distro.toml: %w", err)
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("key must be in section.field format, got %q", key)
	}

	section, field := parts[0], parts[1]
	sectionMap, ok := raw[section].(map[string]interface{})
	if !ok {
		sectionMap = make(map[string]interface{})
		raw[section] = sectionMap
	}
	sectionMap[field] = value

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("writing distro.toml: %w", err)
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(raw)
}
