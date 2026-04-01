package starlark

import "go.starlark.net/starlark"

// Project represents an evaluated PROJECT.star.
type Project struct {
	Name       string
	Version    string
	Defaults   Defaults
	Repository RepositoryConfig
	Cache      CacheConfig
	Sources    SourcesConfig
	Layers     []LayerRef
	Machines   map[string]*Machine
	Units      map[string]*Unit
}

type Defaults struct {
	Machine string
	Image   string
}

type RepositoryConfig struct {
	Path string
}

type CacheConfig struct {
	Path      string
	Remote    []CacheRemote
	Retention int // days
	Signing   string
}

type CacheRemote struct {
	Name     string
	Bucket   string
	Endpoint string
	Region   string
	Prefix   string
}

type SourcesConfig struct {
	GoProxy       string
	CargoRegistry string
	NpmRegistry   string
	PypiMirror    string
}

type LayerRef struct {
	URL   string
	Ref   string
	Path  string // subdirectory within the repo containing LAYER.star
	Local string // local path override (like Go's replace directive)
}

// LayerInfo represents an evaluated LAYER.star from an external layer.
type LayerInfo struct {
	Name        string
	Description string
	Deps        []LayerRef
}

// Machine represents an evaluated machine() call.
type Machine struct {
	Name        string
	Arch        string
	Description string
	Kernel      KernelConfig
	Bootloader  BootloaderConfig
	QEMU        *QEMUConfig // nil if not a QEMU machine
}

type KernelConfig struct {
	Repo        string
	Branch      string
	Tag         string
	Defconfig   string
	DeviceTrees []string
	Unit        string
	Cmdline     string
}

type BootloaderConfig struct {
	Type      string
	Repo      string
	Branch    string
	Defconfig string
}

type QEMUConfig struct {
	Machine  string
	CPU      string
	Memory   string
	Firmware string
	Display  string
}

// Unit represents an evaluated unit(), image(), etc. call.
type Unit struct {
	Name        string
	Version     string
	Class       string // "unit", "image", etc.
	Scope       string // "arch" (default), "machine", or "noarch"
	Description string
	License     string

	// Source
	Source  string // URL or git repo
	SHA256  string
	Tag     string
	Branch  string
	Patches []string // patch files applied after source fetch, before build

	// Dependencies
	Deps        []string
	RuntimeDeps []string

	// Build
	Container string // default container for all tasks
	Tasks     []Task
	Provides  string // virtual package name

	// Artifact metadata
	Services    []string
	Conffiles   []string
	Environment map[string]string

	// Image-specific (class == "image")
	Artifacts  []string // artifacts to install in rootfs
	Exclude    []string
	Hostname   string
	Timezone   string
	Locale     string
	Partitions []Partition
}

type Partition struct {
	Label    string
	Type     string // "vfat", "ext4", etc.
	Size     string // "64M", "fill", etc.
	Root     bool
	Contents []string
}

// Step is a single build action — either a shell command or a Starlark function.
type Step struct {
	Command string            // shell command (set when step is a string)
	Fn      starlark.Callable // Starlark function (set when step is callable)
}

// Task is a named build phase containing one or more steps.
type Task struct {
	Name      string
	Container string // optional container image override
	Steps     []Step
}

// Command represents a user-defined CLI command from commands/*.star.
type Command struct {
	Name        string
	Description string
	Args        []CommandArg
	RunFn       string // name of the run function in the .star file
	SourceFile  string // path to the .star file
}

// CommandArg describes a command-line argument for a custom command.
type CommandArg struct {
	Name     string
	Help     string
	Default  string
	Required bool
	IsBool   bool
}

var validArchitectures = map[string]bool{
	"arm64":   true,
	"riscv64": true,
	"x86_64":  true,
}
