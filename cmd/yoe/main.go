package main

import (
	"fmt"
	"os"

	yoe "github.com/YoeDistro/yoe-ng/internal"
	"github.com/YoeDistro/yoe-ng/internal/resolve"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "init":
		cmdInit(args)
	case "layer":
		cmdLayer(args)
	case "config":
		cmdConfig(args)
	case "desc":
		cmdDesc(args)
	case "refs":
		cmdRefs(args)
	case "graph":
		cmdGraph(args)
	case "clean":
		cmdClean(args)
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s COMMAND [OPTIONS]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Yoe-NG embedded Linux distribution builder\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  init <project-dir>      Create a new Yoe-NG project\n")
	fmt.Fprintf(os.Stderr, "  build [recipes...]      Build recipes (packages and images)\n")
	fmt.Fprintf(os.Stderr, "  flash <device>          Write an image to a device/SD card\n")
	fmt.Fprintf(os.Stderr, "  run                     Run an image in QEMU\n")
	fmt.Fprintf(os.Stderr, "  layer                   Manage external layers (fetch, sync, list)\n")
	fmt.Fprintf(os.Stderr, "  repo                    Manage the local apk package repository\n")
	fmt.Fprintf(os.Stderr, "  cache                   Manage the build cache (local and remote)\n")
	fmt.Fprintf(os.Stderr, "  source                  Download and manage source archives/repos\n")
	fmt.Fprintf(os.Stderr, "  config                  View and edit project configuration\n")
	fmt.Fprintf(os.Stderr, "  desc <recipe>           Describe a recipe or target\n")
	fmt.Fprintf(os.Stderr, "  refs <recipe>           Show reverse dependencies\n")
	fmt.Fprintf(os.Stderr, "  graph                   Visualize the dependency DAG\n")
	fmt.Fprintf(os.Stderr, "  tui                     Launch the interactive TUI\n")
	fmt.Fprintf(os.Stderr, "  clean                   Remove build artifacts\n")
	fmt.Fprintf(os.Stderr, "  version                 Display version information\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  %s init my-project --machine beaglebone-black\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s build openssh\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s build base-image --machine raspberrypi4\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Environment Variables:\n")
	fmt.Fprintf(os.Stderr, "  YOE_PROJECT             Project directory (default: cwd)\n")
	fmt.Fprintf(os.Stderr, "  YOE_CACHE               Cache directory (default: ~/.cache/yoe-ng)\n")
	fmt.Fprintf(os.Stderr, "  YOE_LOG                 Log level: debug, info, warn, error (default: info)\n")
	fmt.Fprintf(os.Stderr, "\n")
}

func cmdLayer(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s layer <sync|list|info> [...]\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}

	switch args[0] {
	case "sync":
		fmt.Fprintf(os.Stderr, "layer sync: not yet implemented\n")
		os.Exit(1)
	case "list":
		if err := yoe.ListLayers(dir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "info":
		fmt.Fprintf(os.Stderr, "layer info: not yet implemented\n")
		os.Exit(1)
	case "check-updates":
		fmt.Fprintf(os.Stderr, "layer check-updates: not yet implemented\n")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "Unknown layer subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdInit(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s init <project-dir> [-machine <name>]\n", os.Args[0])
		os.Exit(1)
	}

	projectDir := args[0]
	machine := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-machine", "--machine":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: -machine requires a name\n")
				os.Exit(1)
			}
			machine = args[i+1]
			i++
		default:
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	if err := yoe.RunInit(projectDir, machine); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdConfig(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s config <show|set> [...]\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}

	switch args[0] {
	case "show":
		if err := yoe.ShowConfig(dir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "set":
		fmt.Fprintf(os.Stderr, "config set: edit PROJECT.star directly (Starlark files are not patchable via CLI)\n")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdClean(args []string) {
	all := false
	var recipes []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-all", "--all":
			all = true
		default:
			recipes = append(recipes, args[i])
		}
	}

	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}

	if err := yoe.RunClean(dir, all, recipes); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadProject() *yoestar.Project {
	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}
	proj, err := yoestar.LoadProject(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return proj
}

func defaultArch(proj *yoestar.Project) string {
	if m, ok := proj.Machines[proj.Defaults.Machine]; ok {
		return m.Arch
	}
	// Fallback: pick the first machine's arch
	for _, m := range proj.Machines {
		return m.Arch
	}
	return "unknown"
}

func cmdDesc(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s desc <recipe>\n", os.Args[0])
		os.Exit(1)
	}
	proj := loadProject()
	arch := defaultArch(proj)
	if err := resolve.Describe(os.Stdout, proj, args[0], arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdRefs(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s refs <recipe> [--direct]\n", os.Args[0])
		os.Exit(1)
	}
	name := args[0]
	direct := false
	for _, a := range args[1:] {
		if a == "--direct" || a == "-direct" {
			direct = true
		}
	}
	proj := loadProject()
	if err := resolve.Refs(os.Stdout, proj, name, direct); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdGraph(args []string) {
	format := "text"
	filter := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format", "-format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		default:
			filter = args[i]
		}
	}
	proj := loadProject()
	if err := resolve.Graph(os.Stdout, proj, format, filter); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
