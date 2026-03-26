package source

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// FetchAll downloads sources for all recipes (or specific ones).
func FetchAll(projectDir string, recipeNames []string, w io.Writer) error {
	proj, err := yoestar.LoadProject(projectDir)
	if err != nil {
		return err
	}

	recipes := filterRecipes(proj, recipeNames)
	for _, recipe := range recipes {
		if recipe.Source == "" {
			continue
		}
		if _, err := Fetch(recipe); err != nil {
			return fmt.Errorf("fetching %s: %w", recipe.Name, err)
		}
	}

	return nil
}

// ListSources shows cached sources and their status.
func ListSources(projectDir string, w io.Writer) error {
	proj, err := yoestar.LoadProject(projectDir)
	if err != nil {
		return err
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "%-20s %-10s %s\n", "Recipe", "Status", "Source")
	for _, recipe := range proj.Recipes {
		if recipe.Source == "" {
			continue
		}

		status := "missing"
		if isCached(cacheDir, recipe) {
			status = "cached"
		}

		src := recipe.Source
		if len(src) > 60 {
			src = src[:57] + "..."
		}
		fmt.Fprintf(w, "%-20s %-10s %s\n", recipe.Name, status, src)
	}

	return nil
}

// VerifyAll checks SHA256 of cached sources.
func VerifyAll(projectDir string, w io.Writer) error {
	proj, err := yoestar.LoadProject(projectDir)
	if err != nil {
		return err
	}

	allOk := true
	for _, recipe := range proj.Recipes {
		if recipe.Source == "" || recipe.SHA256 == "" {
			continue
		}
		if err := Verify(recipe); err != nil {
			fmt.Fprintf(w, "FAIL  %s: %v\n", recipe.Name, err)
			allOk = false
		} else {
			fmt.Fprintf(w, "OK    %s\n", recipe.Name)
		}
	}

	if !allOk {
		return fmt.Errorf("some sources failed verification")
	}
	return nil
}

// CleanSources removes cached sources.
func CleanSources(w io.Writer) error {
	cacheDir, err := CacheDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(w, "No cached sources")
			return nil
		}
		return err
	}

	count := 0
	for _, e := range entries {
		path := filepath.Join(cacheDir, e.Name())
		os.RemoveAll(path)
		count++
	}

	fmt.Fprintf(w, "Removed %d cached source(s)\n", count)
	return nil
}

func filterRecipes(proj *yoestar.Project, names []string) []*yoestar.Recipe {
	if len(names) == 0 {
		result := make([]*yoestar.Recipe, 0, len(proj.Recipes))
		for _, r := range proj.Recipes {
			result = append(result, r)
		}
		return result
	}

	result := make([]*yoestar.Recipe, 0, len(names))
	for _, name := range names {
		if r, ok := proj.Recipes[name]; ok {
			result = append(result, r)
		}
	}
	return result
}

func isCached(cacheDir string, recipe *yoestar.Recipe) bool {
	urlHash := fmt.Sprintf("%x", sha256.Sum256([]byte(recipe.Source)))
	if isGitURL(recipe.Source) {
		_, err := os.Stat(filepath.Join(cacheDir, urlHash+".git"))
		return err == nil
	}
	ext := guessExt(recipe.Source)
	_, err := os.Stat(filepath.Join(cacheDir, urlHash+ext))
	return err == nil
}
