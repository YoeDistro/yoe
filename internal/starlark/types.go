package starlark

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
	Recipes    map[string]*Recipe
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
	URL string
	Ref string
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
	Recipe      string
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

// Recipe represents an evaluated package(), autotools(), image(), etc. call.
type Recipe struct {
	Name        string
	Version     string
	Class       string // "package", "autotools", "cmake", "go", "image", etc.
	Description string
	License     string

	// Source
	Source string // URL or git repo
	SHA256 string
	Tag    string
	Branch string

	// Dependencies
	Deps        []string
	RuntimeDeps []string

	// Build
	Build         []string // shell commands (for generic package())
	ConfigureArgs []string // for autotools/cmake
	GoPackage     string   // for go_binary

	// Package metadata
	Services    []string
	Conffiles   []string
	Environment map[string]string

	// Image-specific (class == "image")
	Packages   []string // packages to install in rootfs
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

var validArchitectures = map[string]bool{
	"arm64":   true,
	"riscv64": true,
	"x86_64":  true,
}
