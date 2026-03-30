package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

func RunClean(projectDir string, all bool, units []string) error {
	buildDir := filepath.Join(projectDir, "build")

	if len(units) > 0 {
		for _, r := range units {
			dir := filepath.Join(buildDir, r)
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("removing %s: %w", dir, err)
			}
			fmt.Printf("Cleaned %s\n", r)
		}
		return nil
	}

	if all {
		dirs := []string{buildDir, filepath.Join(projectDir, "repo")}
		for _, dir := range dirs {
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("removing %s: %w", dir, err)
			}
		}
		fmt.Println("Cleaned all build artifacts, packages, and sources")
	} else {
		if err := os.RemoveAll(buildDir); err != nil {
			return fmt.Errorf("removing %s: %w", buildDir, err)
		}
		fmt.Println("Cleaned build intermediates (packages preserved)")
	}

	return nil
}
