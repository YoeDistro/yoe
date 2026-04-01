package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"

	yoe "github.com/YoeDistro/yoe-ng/internal"
	"github.com/YoeDistro/yoe-ng/internal/bootstrap"
	"github.com/YoeDistro/yoe-ng/internal/build"
	"github.com/YoeDistro/yoe-ng/internal/device"
	"github.com/YoeDistro/yoe-ng/internal/layer"
	"github.com/YoeDistro/yoe-ng/internal/repo"
	"github.com/YoeDistro/yoe-ng/internal/resolve"
	"github.com/YoeDistro/yoe-ng/internal/source"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
	"github.com/YoeDistro/yoe-ng/internal/tui"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		cmdTUI(nil)
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "version":
		fmt.Println(version)
	case "update":
		cmdUpdate()
	case "init":
		cmdInit(args)
	case "container":
		cmdContainer(args)
	case "layer":
		cmdLayer(args)
	case "build":
		cmdBuild(args)
	case "bootstrap":
		cmdBootstrap(args)
	case "flash":
		cmdFlash(args)
	case "run":
		cmdRun(args)
	case "config":
		cmdConfig(args)
	case "repo":
		cmdRepo(args)
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
	case "log":
		cmdLog(args)
	case "diagnose":
		cmdDiagnose(args)
	case "clean":
		cmdClean(args)
	default:
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
	fmt.Fprintf(os.Stderr, "  (no args)               Launch the interactive TUI\n")
	fmt.Fprintf(os.Stderr, "  init <project-dir>      Create a new Yoe-NG project\n")
	fmt.Fprintf(os.Stderr, "  container               Manage the build container (build, shell, status)\n")
	fmt.Fprintf(os.Stderr, "  build [units...]      Build units (--force, --clean, --verbose, --dry-run)\n")
	fmt.Fprintf(os.Stderr, "  dev                     Manage source modifications (extract, diff, status)\n")
	fmt.Fprintf(os.Stderr, "  flash <device>          Write an image to a device/SD card\n")
	fmt.Fprintf(os.Stderr, "  run                     Run an image in QEMU\n")
	fmt.Fprintf(os.Stderr, "  layer                   Manage external layers (fetch, sync, list)\n")
	fmt.Fprintf(os.Stderr, "  repo                    Manage the local apk package repository\n")
	fmt.Fprintf(os.Stderr, "  cache                   Manage the build cache (local and remote)\n")
	fmt.Fprintf(os.Stderr, "  source                  Download and manage source archives/repos\n")
	fmt.Fprintf(os.Stderr, "  config                  View and edit project configuration\n")
	fmt.Fprintf(os.Stderr, "  desc <unit>           Describe a unit or target\n")
	fmt.Fprintf(os.Stderr, "  refs <unit>           Show reverse dependencies\n")
	fmt.Fprintf(os.Stderr, "  graph                   Visualize the dependency DAG\n")
	fmt.Fprintf(os.Stderr, "  log [unit] [-e]         Show build log (most recent, or specific unit; -e to edit)\n")
	fmt.Fprintf(os.Stderr, "  diagnose [unit]         Launch Claude Code to diagnose a build failure\n")
	fmt.Fprintf(os.Stderr, "  clean                   Remove build artifacts\n")
	fmt.Fprintf(os.Stderr, "  update                  Update yoe to the latest release\n")
	fmt.Fprintf(os.Stderr, "  version                 Display version information\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  %s init my-project --machine beaglebone-black\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s build openssh\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s build base-image --machine raspberrypi4\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Environment Variables:\n")
	fmt.Fprintf(os.Stderr, "  YOE_PROJECT             Project directory (default: cwd)\n")
	fmt.Fprintf(os.Stderr, "  YOE_CACHE               Cache directory (default: cache/ in project dir)\n")
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
		proj := loadProject()
		if _, err := layer.Sync(proj, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
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

func resolveTargetArch(proj *yoestar.Project, machineName string) (string, error) {
	if machineName != "" {
		m, ok := proj.Machines[machineName]
		if !ok {
			return "", fmt.Errorf("machine %q not found", machineName)
		}
		return m.Arch, nil
	}
	// Use the default machine's arch
	if m, ok := proj.Machines[proj.Defaults.Machine]; ok {
		return m.Arch, nil
	}
	// Fallback to host arch
	return build.Arch(), nil
}

func cmdBuild(args []string) {
	force := false
	clean := false
	noCache := false
	dryRun := false
	verbose := false
	machineName := ""
	var units []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force", "-force":
			force = true
		case "--clean":
			clean = true
		case "--no-cache":
			noCache = true
		case "--dry-run":
			dryRun = true
		case "--verbose", "-v":
			verbose = true
		case "--machine":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		case "--all":
			// build all — units stays empty
		default:
			units = append(units, args[i])
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	proj := loadProjectWithMachine(machineName)
	targetArch, err := resolveTargetArch(proj, machineName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	resolvedMachine := machineName
	if resolvedMachine == "" {
		resolvedMachine = proj.Defaults.Machine
	}
	opts := build.Options{
		Ctx:        ctx,
		Force:      force,
		Clean:      clean,
		NoCache:    noCache,
		DryRun:     dryRun,
		Verbose:    verbose,
		ProjectDir: projectDir(),
		Arch:       targetArch,
		Machine:    resolvedMachine,
	}

	if err := build.BuildUnits(proj, units, opts, os.Stdout); err != nil {
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
		fmt.Fprintf(os.Stderr, "Usage: %s container <build|shell|status|binfmt>\n", os.Args[0])
		os.Exit(1)
	}

	switch args[0] {
	case "build":
		if err := yoe.EnsureImage("", os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Container image yoe-ng:%s built successfully\n", yoe.ContainerVersion())
	case "shell":
		cmdContainerShell()
	case "status":
		fmt.Printf("Container version: %s (image: yoe-ng:%s)\n", yoe.ContainerVersion(), yoe.ContainerVersion())
		if err := yoe.EnsureImage("", io.Discard); err != nil {
			fmt.Println("Container image: not built")
		} else {
			fmt.Println("Container image: ready")
		}
	case "binfmt":
		fmt.Println("This will register QEMU user-mode emulation for foreign architectures")
		fmt.Println("by running a privileged Docker container (tonistiigi/binfmt).")
		fmt.Println()
		fmt.Println("This enables building arm64 and riscv64 images on your " + build.Arch() + " host.")
		fmt.Println("The registration persists until reboot.")
		fmt.Println()
		fmt.Print("Proceed? (y/n) ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Cancelled.")
			return
		}
		if err := yoe.RegisterBinfmt(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown container subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdContainerShell() {
	projectDir := projectDir()
	sysroot := filepath.Join(projectDir, "build", build.Arch(), "shell", "sysroot")
	build.EnsureDir(sysroot)

	// Use a temp dir for src/destdir so the sandbox mounts are valid
	srcDir := filepath.Join(projectDir, "build", build.Arch(), "shell", "src")
	destDir := filepath.Join(projectDir, "build", build.Arch(), "shell", "destdir")
	build.EnsureDir(srcDir)
	build.EnsureDir(destDir)

	cfg := &build.SandboxConfig{
		SrcDir:     srcDir,
		DestDir:    destDir,
		Sysroot:    sysroot,
		ProjectDir: projectDir,
		Env: map[string]string{
			"PREFIX":          "/usr",
			"DESTDIR":         "/build/destdir",
			"NPROC":           build.NProc(),
			"ARCH":            build.Arch(),
			"HOME":            "/tmp",
			"PATH":            "/build/sysroot/usr/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"PKG_CONFIG_PATH": "/build/sysroot/usr/lib/pkgconfig:/usr/lib/pkgconfig",
			"CFLAGS":          "-I/build/sysroot/usr/include",
			"CPPFLAGS":        "-I/build/sysroot/usr/include",
			"LDFLAGS":         "-L/build/sysroot/usr/lib",
			"PYTHONPATH":      "/build/sysroot/usr/lib/python3.12/site-packages",
		},
	}

	bwrapCmd := build.BwrapShellCommand(cfg)
	mounts := []yoe.Mount{
		{Host: srcDir, Container: "/build/src"},
		{Host: destDir, Container: "/build/destdir"},
		{Host: sysroot, Container: "/build/sysroot", ReadOnly: true},
	}

	if err := yoe.RunInContainer(yoe.ContainerRunConfig{
		Command:     bwrapCmd,
		ProjectDir:  projectDir,
		Mounts:      mounts,
		Interactive: true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
	force := false
	locks := false
	var units []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-all", "--all":
			all = true
		case "--force", "-f":
			force = true
		case "--locks":
			locks = true
		default:
			units = append(units, args[i])
		}
	}

	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}

	if locks {
		if err := yoe.CleanLocks(dir, build.Arch()); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := yoe.RunClean(dir, build.Arch(), all, force, units); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadProject() *yoestar.Project {
	return loadProjectWithMachine("")
}

func loadProjectWithMachine(machineName string) *yoestar.Project {
	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}
	opts := []yoestar.LoadOption{
		yoestar.WithLayerSync(layer.SyncIfNeeded),
	}
	if machineName != "" {
		opts = append(opts, yoestar.WithMachine(machineName))
	}
	proj, err := yoestar.LoadProject(dir, opts...)
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
		fmt.Fprintf(os.Stderr, "Usage: %s desc <unit>\n", os.Args[0])
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
		fmt.Fprintf(os.Stderr, "Usage: %s refs <unit> [--direct]\n", os.Args[0])
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
		fmt.Fprintf(os.Stderr, "Usage: %s dev <extract|diff|status> [unit]\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Getenv("YOE_PROJECT")
	if dir == "" {
		dir = "."
	}

	switch args[0] {
	case "extract":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s dev extract <unit>\n", os.Args[0])
			os.Exit(1)
		}
		if err := yoe.DevExtract(dir, build.Arch(), args[1], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "diff":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s dev diff <unit>\n", os.Args[0])
			os.Exit(1)
		}
		if err := yoe.DevDiff(dir, build.Arch(), args[1], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := yoe.DevStatus(dir, build.Arch(), os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown dev subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdBootstrap(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s bootstrap <stage0|stage1|status>\n", os.Args[0])
		os.Exit(1)
	}

	proj := loadProject()
	dir := projectDir()

	switch args[0] {
	case "stage0":
		if err := bootstrap.Stage0(proj, dir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "stage1":
		if err := bootstrap.Stage1(proj, dir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := bootstrap.Status(proj, dir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown bootstrap subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdLog(args []string) {
	edit := false
	var unitName string

	for _, a := range args {
		switch a {
		case "-e", "--edit":
			edit = true
		default:
			unitName = a
		}
	}

	dir := projectDir()
	var logPath string

	if unitName != "" {
		logPath = filepath.Join(build.UnitBuildDir(dir, build.Arch(), unitName), "build.log")
	} else {
		logPath = findLatestBuildLog(dir)
	}

	if logPath == "" {
		fmt.Fprintln(os.Stderr, "No build logs found")
		os.Exit(1)
	}

	if edit {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		cmd := exec.Command(editor, logPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(data)
}

func cmdDiagnose(args []string) {
	var unitName string
	for _, a := range args {
		unitName = a
	}

	dir := projectDir()
	var logPath string

	if unitName != "" {
		logPath = filepath.Join(build.UnitBuildDir(dir, build.Arch(), unitName), "build.log")
	} else {
		logPath = findLatestBuildLog(dir)
	}

	if logPath == "" {
		fmt.Fprintln(os.Stderr, "No build logs found")
		os.Exit(1)
	}

	if _, err := os.Stat(logPath); err != nil {
		fmt.Fprintf(os.Stderr, "Build log not found: %s\n", logPath)
		os.Exit(1)
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: claude not found in PATH")
		os.Exit(1)
	}

	prompt := fmt.Sprintf("diagnose %s", logPath)
	cmd := exec.Command(claudePath, prompt)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func findLatestBuildLog(projectDir string) string {
	archDir := filepath.Join(projectDir, "build", build.Arch())
	entries, err := os.ReadDir(archDir)
	if err != nil {
		return ""
	}

	type logEntry struct {
		path    string
		modTime int64
	}
	var logs []logEntry

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(archDir, e.Name(), "build.log")
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		logs = append(logs, logEntry{p, info.ModTime().UnixNano()})
	}

	if len(logs) == 0 {
		return ""
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].modTime > logs[j].modTime
	})
	return logs[0].path
}

func cmdUpdate() {
	if err := yoe.Update(version); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
}

func cmdTUI(_ []string) {
	proj := loadProject()
	if err := tui.Run(proj, projectDir()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdFlash(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s flash <image-unit> <device> [--dry-run]\n", os.Args[0])
		os.Exit(1)
	}

	unitName := args[0]
	devicePath := ""
	dryRun := false

	for _, a := range args[1:] {
		switch a {
		case "--dry-run":
			dryRun = true
		default:
			devicePath = a
		}
	}

	if devicePath == "" && !dryRun {
		fmt.Fprintf(os.Stderr, "Usage: %s flash <image-unit> <device>\n", os.Args[0])
		os.Exit(1)
	}

	proj := loadProject()
	if err := device.Flash(proj, unitName, devicePath, projectDir(), dryRun, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdRun(args []string) {
	unitName := ""
	machineName := ""
	opts := device.QEMUOptions{Memory: "1G"}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--machine", "-machine":
			if i+1 < len(args) {
				machineName = args[i+1]
				i++
			}
		case "--memory":
			if i+1 < len(args) {
				opts.Memory = args[i+1]
				i++
			}
		case "--port":
			if i+1 < len(args) {
				opts.Ports = append(opts.Ports, args[i+1])
				i++
			}
		case "--display":
			opts.Display = true
		case "--daemon":
			opts.Daemon = true
		default:
			unitName = args[i]
		}
	}

	proj := loadProject()
	if unitName == "" {
		unitName = proj.Defaults.Image
	}
	if unitName == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s run <image-unit> [--machine <name>]\n", os.Args[0])
		os.Exit(1)
	}

	if err := device.RunQEMU(proj, unitName, machineName, projectDir(), opts, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdRepo(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s repo <list|info|remove> [args...]\n", os.Args[0])
		os.Exit(1)
	}

	proj := loadProject()
	repoDir := repo.RepoDir(proj, projectDir())

	switch args[0] {
	case "list":
		if err := repo.List(repoDir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "info":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s repo info <package>\n", os.Args[0])
			os.Exit(1)
		}
		if err := repo.Info(repoDir, args[1], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "remove":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s repo remove <package>\n", os.Args[0])
			os.Exit(1)
		}
		if err := repo.Remove(repoDir, args[1], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown repo subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdSource(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s source <fetch|list|verify|clean> [units...]\n", os.Args[0])
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
