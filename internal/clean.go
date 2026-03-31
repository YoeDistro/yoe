package internal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RunClean(projectDir string, all bool, force bool, units []string) error {
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
		if !force {
			fmt.Print("Remove all build artifacts and packages? [y/N] ")
			if !confirmYes() {
				fmt.Println("Aborted")
				return nil
			}
		}
		dirs := []string{buildDir, filepath.Join(projectDir, "repo")}
		for _, dir := range dirs {
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("removing %s: %w", dir, err)
			}
		}
		fmt.Println("Cleaned all build artifacts, packages, and sources")
	} else {
		if !force {
			fmt.Print("Remove all build intermediates? [y/N] ")
			if !confirmYes() {
				fmt.Println("Aborted")
				return nil
			}
		}
		if err := os.RemoveAll(buildDir); err != nil {
			return fmt.Errorf("removing %s: %w", buildDir, err)
		}
		fmt.Println("Cleaned build intermediates (packages preserved)")
	}

	return nil
}

func CleanLocks(projectDir string) error {
	buildDir := filepath.Join(projectDir, "build")
	entries, err := os.ReadDir(buildDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No build directory")
			return nil
		}
		return err
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		lockPath := filepath.Join(buildDir, e.Name(), ".lock")
		if _, err := os.Stat(lockPath); err == nil {
			os.Remove(lockPath)
			fmt.Printf("Removed lock: %s\n", e.Name())
			count++
		}
	}
	if count == 0 {
		fmt.Println("No stale locks found")
	} else {
		fmt.Printf("Removed %d lock(s)\n", count)
	}
	return nil
}

func confirmYes() bool {
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(scanner.Text()), "y")
}
