package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	yoe "github.com/YoeDistro/yoe-ng/internal"
	"github.com/YoeDistro/yoe-ng/internal/build"
	"github.com/YoeDistro/yoe-ng/internal/resolve"
	"github.com/YoeDistro/yoe-ng/internal/source"
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

	// Commands that run on the host without a container
	switch command {
	case "version":
		fmt.Println(version)
		return
	case "init":
		cmdInit(args)
		return
	case "container":
		cmdContainer(args)
		return
	}

	// Everything else runs inside the container
	if !yoe.InContainer() {
		if err := yoe.EnsureImage(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := yoe.ExecInContainer(os.Args[1:]); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Inside container — dispatch commands
	switch command {
	case "build":
		cmdBuild(args)
	case "layer":
		cmdLayer(args)
	case "config":
		cmdConfig(args)
	case "source":
		cmdSource(args)
	case "dev":
		cmdDev(args)
	case "desc":
		cmdDesc(args)
	case "refs":
		cmdRefs(args)
	case "graph":
		cmdGraph(args)
	case "clean":
		cmdClean(args)
	default:
		// Check for custom commands from commands/*.star
		if !tryCustomCommand(command, args) {
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s COMMAND [OPTIONS]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Yoe-NG embedded Linux distribution builder\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  init <project-dir>      Create a new Yoe-NG project\n")
	fmt.Fprintf(os.Stderr, "  container               Manage the build container (build, status)\n")
	fmt.Fprintf(os.Stderr, "  build [recipes...]      Build recipes (packages and images)\n")
	fmt.Fprintf(os.Stderr, "  dev                     Manage source modifications (extract, diff, status)\n")
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

func cmdBuild(args []string) {
	force := false
	noCache := false
	dryRun := false
	var recipes []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force", "-force":
			force = true
		case "--no-cache":
			noCache = true
		case "--dry-run":
			dryRun = true
		case "--all":
			// build all — recipes stays empty
		default:
			recipes = append(recipes, args[i])
		}
	}

	proj := loadProject()
	opts := build.Options{
		Force:      force,
		NoCache:    noCache,
		DryRun:     dryRun,
		UseSandbox: build.HasBwrap(),
		ProjectDir: projectDir(),
		Arch:       build.Arch(),
	}

	if err := build.BuildRecipes(proj, recipes, opts, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func projectDir() string {
	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

func cmdContainer(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s container <build|status>\n", os.Args[0])
		os.Exit(1)
	}

	switch args[0] {
	case "build":
		if err := yoe.EnsureImage(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Container image yoe-ng:%s built successfully\n", yoe.ContainerVersion())
	case "status":
		fmt.Printf("Container version: %s (image: yoe-ng:%s)\n", yoe.ContainerVersion(), yoe.ContainerVersion())
		if yoe.InContainer() {
			fmt.Println("Currently running inside the yoe-ng container")
		} else {
			fmt.Println("Running on host — commands will auto-enter container")
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown container subcommand: %s\n", args[0])
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

func cmdDev(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s dev <extract|diff|status> [recipe]\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}

	switch args[0] {
	case "extract":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s dev extract <recipe>\n", os.Args[0])
			os.Exit(1)
		}
		if err := yoe.DevExtract(dir, args[1], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "diff":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s dev diff <recipe>\n", os.Args[0])
			os.Exit(1)
		}
		if err := yoe.DevDiff(dir, args[1], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := yoe.DevStatus(dir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown dev subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdSource(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s source <fetch|list|verify|clean> [recipes...]\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}

	switch args[0] {
	case "fetch":
		if err := source.FetchAll(dir, args[1:], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "list":
		if err := source.ListSources(dir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "verify":
		if err := source.VerifyAll(dir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "clean":
		if err := source.CleanSources(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown source subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

// tryCustomCommand checks for a custom command in commands/*.star and runs it.
// Returns true if the command was found and executed.
func tryCustomCommand(command string, args []string) bool {
	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}

	cmds, engines, err := yoestar.LoadCommands(dir)
	if err != nil {
		// No commands directory or eval error — not a custom command
		return false
	}

	cmd, ok := cmds[command]
	if !ok {
		return false
	}

	eng := engines[command]
	if err := yoestar.RunCommand(eng, cmd, args, dir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return true
}
