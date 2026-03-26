package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

func RunClean(dir string, all bool) error {
	buildDir := filepath.Join(dir, "build")
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("removing build directory: %w", err)
	}
	fmt.Println("Removed build directory")

	if all {
		for _, subdir := range []string{"packages", "sources"} {
			path := filepath.Join(dir, subdir)
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("removing %s: %w", subdir, err)
			}
			fmt.Printf("Removed %s directory\n", subdir)
		}
	}

	return nil
}
