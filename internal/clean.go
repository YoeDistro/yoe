package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

func RunClean(projectDir string, all bool, recipes []string) error {
	buildDir := filepath.Join(projectDir, "build")

	if len(recipes) > 0 {
		for _, r := range recipes {
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
