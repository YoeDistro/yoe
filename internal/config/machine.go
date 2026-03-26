package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

var validArchitectures = map[string]bool{
	"arm":     true,
	"arm64":   true,
	"riscv64": true,
	"x86_64":  true,
}

type MachineConfig struct {
	Machine    MachineInfo  `toml:"machine"`
	Kernel     KernelConfig `toml:"kernel"`
	Bootloader BootConfig   `toml:"bootloader"`
	Image      MachineImage `toml:"image"`
}

type MachineInfo struct {
	Name        string      `toml:"name"`
	Arch        string      `toml:"arch"`
	Description string      `toml:"description"`
	QEMU        *QEMUConfig `toml:"qemu"`
}

type KernelConfig struct {
	Repo        string   `toml:"repo"`
	Branch      string   `toml:"branch"`
	Tag         string   `toml:"tag"`
	Defconfig   string   `toml:"defconfig"`
	DeviceTrees []string `toml:"device-trees"`
	Recipe      string   `toml:"recipe"`
	Cmdline     string   `toml:"cmdline"`
}

type BootConfig struct {
	Type      string `toml:"type"`
	Repo      string `toml:"repo"`
	Branch    string `toml:"branch"`
	Defconfig string `toml:"defconfig"`
}

type MachineImage struct {
	PartitionLayout string `toml:"partition-layout"`
}

type QEMUConfig struct {
	Machine  string `toml:"machine"`
	CPU      string `toml:"cpu"`
	Memory   string `toml:"memory"`
	Firmware string `toml:"firmware"`
	Display  string `toml:"display"`
}

func ParseMachineConfig(path string) (*MachineConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading machine config: %w", err)
	}

	var cfg MachineConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing machine config %s: %w", path, err)
	}

	if cfg.Machine.Name == "" {
		return nil, fmt.Errorf("machine config %s: machine.name is required", path)
	}
	if cfg.Machine.Arch == "" {
		return nil, fmt.Errorf("machine config %s: machine.arch is required", path)
	}
	if !validArchitectures[cfg.Machine.Arch] {
		return nil, fmt.Errorf("machine config %s: invalid arch %q (valid: arm, arm64, riscv64, x86_64)", path, cfg.Machine.Arch)
	}

	return &cfg, nil
}
