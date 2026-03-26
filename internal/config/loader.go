package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Project struct {
	Root       string
	Distro     *DistroConfig
	Machines   map[string]*MachineConfig
	Images     map[string]*ImageConfig
	Recipes    map[string]*RecipeConfig
	Partitions map[string]*PartitionConfig
}

func LoadProject(dir string) (*Project, error) {
	root, err := FindProjectRoot(dir)
	if err != nil {
		return nil, err
	}

	distro, err := ParseDistroConfig(filepath.Join(root, "distro.toml"))
	if err != nil {
		return nil, err
	}

	project := &Project{
		Root:       root,
		Distro:     distro,
		Machines:   make(map[string]*MachineConfig),
		Images:     make(map[string]*ImageConfig),
		Recipes:    make(map[string]*RecipeConfig),
		Partitions: make(map[string]*PartitionConfig),
	}

	if err := project.loadDir("machines", func(path string) error {
		m, err := ParseMachineConfig(path)
		if err != nil {
			return err
		}
		project.Machines[m.Machine.Name] = m
		return nil
	}); err != nil {
		return nil, err
	}

	if err := project.loadDir("images", func(path string) error {
		img, err := ParseImageConfig(path)
		if err != nil {
			return err
		}
		project.Images[img.Image.Name] = img
		return nil
	}); err != nil {
		return nil, err
	}

	if err := project.loadDir("recipes", func(path string) error {
		r, err := ParseRecipeConfig(path)
		if err != nil {
			return err
		}
		project.Recipes[r.Recipe.Name] = r
		return nil
	}); err != nil {
		return nil, err
	}

	if err := project.loadDir("partitions", func(path string) error {
		p, err := ParsePartitionConfig(path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".toml")
		project.Partitions[name] = p
		return nil
	}); err != nil {
		return nil, err
	}

	return project, nil
}

func (p *Project) loadDir(subdir string, load func(string) error) error {
	pattern := filepath.Join(p.Root, subdir, "*.toml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("globbing %s: %w", pattern, err)
	}
	for _, path := range matches {
		if err := load(path); err != nil {
			return err
		}
	}
	return nil
}
