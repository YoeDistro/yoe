package internal

import (
	"fmt"
	"io"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

func ListLayers(dir string, w io.Writer) error {
	proj, err := yoestar.LoadProject(dir)
	if err != nil {
		return err
	}

	if len(proj.Layers) == 0 {
		fmt.Fprintln(w, "No layers declared in PROJECT.star")
		return nil
	}

	fmt.Fprintf(w, "%-40s %-12s %s\n", "Layer", "Ref", "Status")
	for _, l := range proj.Layers {
		status := "not synced"
		if l.Local != "" {
			status = fmt.Sprintf("(local: %s)", l.Local)
		}
		ref := l.Ref
		if ref == "" {
			ref = "(none)"
		}
		fmt.Fprintf(w, "%-40s %-12s %s\n", l.URL, ref, status)
	}

	return nil
}
