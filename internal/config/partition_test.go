package config

import (
	"testing"
)

func TestParsePartitionConfig(t *testing.T) {
	cfg, err := ParsePartitionConfig("../../testdata/valid-project/partitions/bbb.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Disk.Type != "gpt" {
		t.Errorf("expected disk type %q, got %q", "gpt", cfg.Disk.Type)
	}
	if len(cfg.Partition) != 2 {
		t.Fatalf("expected 2 partitions, got %d", len(cfg.Partition))
	}
	if cfg.Partition[0].Label != "boot" {
		t.Errorf("expected first partition label %q, got %q", "boot", cfg.Partition[0].Label)
	}
	if cfg.Partition[1].Root != true {
		t.Errorf("expected rootfs partition to have root=true")
	}
}

func TestParsePartitionConfig_NoRootPartition(t *testing.T) {
	_, err := ParsePartitionConfig("../../testdata/invalid-project/no-root-partition.toml")
	if err == nil {
		t.Fatal("expected error for partition config with no root partition")
	}
}
