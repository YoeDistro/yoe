package main

import (
	"fmt"
	"os"
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
	case "config":
		cmdConfig(args)
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
	fmt.Fprintf(os.Stderr, "  build [recipes...]      Build packages from recipes\n")
	fmt.Fprintf(os.Stderr, "  image                   Assemble a root filesystem image\n")
	fmt.Fprintf(os.Stderr, "  flash <device>          Write an image to a device/SD card\n")
	fmt.Fprintf(os.Stderr, "  run                     Run an image in QEMU\n")
	fmt.Fprintf(os.Stderr, "  repo                    Manage the local apk package repository\n")
	fmt.Fprintf(os.Stderr, "  source                  Download and manage source archives/repos\n")
	fmt.Fprintf(os.Stderr, "  config                  View and edit project configuration\n")
	fmt.Fprintf(os.Stderr, "  desc <recipe>           Describe a recipe or target\n")
	fmt.Fprintf(os.Stderr, "  refs <recipe>           Show reverse dependencies\n")
	fmt.Fprintf(os.Stderr, "  graph                   Visualize the dependency DAG\n")
	fmt.Fprintf(os.Stderr, "  tui                     Launch the interactive TUI\n")
	fmt.Fprintf(os.Stderr, "  clean                   Remove build artifacts\n")
	fmt.Fprintf(os.Stderr, "  version                 Display version information\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Init Options:\n")
	fmt.Fprintf(os.Stderr, "  -machine <name>         Initial machine to configure\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Config Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  config show             Show current configuration\n")
	fmt.Fprintf(os.Stderr, "  config set <key> <val>  Set a configuration value\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Clean Options:\n")
	fmt.Fprintf(os.Stderr, "  -all                    Remove everything (build dirs, packages, sources)\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Environment Variables:\n")
	fmt.Fprintf(os.Stderr, "  YOE_PROJECT             Project directory (default: cwd)\n")
	fmt.Fprintf(os.Stderr, "  YOE_CACHE               Cache directory (default: ~/.cache/yoe-ng)\n")
	fmt.Fprintf(os.Stderr, "  YOE_LOG                 Log level: debug, info, warn, error (default: info)\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  %s init my-project --machine beaglebone-black\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s build openssh\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s image\n", os.Args[0])
}

// Stub command handlers — implemented in subsequent tasks

func cmdInit(args []string) {
	fmt.Fprintf(os.Stderr, "init: not yet implemented\n")
	os.Exit(1)
}

func cmdConfig(args []string) {
	fmt.Fprintf(os.Stderr, "config: not yet implemented\n")
	os.Exit(1)
}

func cmdClean(args []string) {
	fmt.Fprintf(os.Stderr, "clean: not yet implemented\n")
	os.Exit(1)
}
