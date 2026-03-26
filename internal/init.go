package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

var distroTemplate = `[distro]
name = "{{.Name}}"
version = "0.1.0"
description = ""

[defaults]
machine = "{{.Machine}}"
image = "base"

[repository]
path = "/var/cache/yoe-ng/repo"

[cache]
path = "/var/cache/yoe-ng/build"

[sources]
go-proxy = "https://proxy.golang.org"
`

var qemuMachineTemplate = `[machine]
name = "{{.Name}}"
arch = "{{.Arch}}"

[kernel]
recipe = "linux-qemu"
cmdline = "console={{.Console}} root=/dev/vda2 rw"

[machine.qemu]
machine = "{{.QEMUMachine}}"
cpu = "host"
memory = "1G"
firmware = "{{.Firmware}}"
display = "none"
`

func RunInit(projectDir string, machine string) error {
	if _, err := os.Stat(filepath.Join(projectDir, "distro.toml")); err == nil {
		return fmt.Errorf("project already exists at %s (distro.toml found)", projectDir)
	}

	dirs := []string{"machines", "images", "recipes", "partitions", "overlays"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(projectDir, dir), 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	name := filepath.Base(projectDir)
	defaultMachine := machine
	if defaultMachine == "" {
		defaultMachine = "qemu-x86_64"
	}

	tmpl, err := template.New("distro").Parse(distroTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	f, err := os.Create(filepath.Join(projectDir, "distro.toml"))
	if err != nil {
		return fmt.Errorf("creating distro.toml: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, map[string]string{
		"Name":    name,
		"Machine": defaultMachine,
	}); err != nil {
		return fmt.Errorf("writing distro.toml: %w", err)
	}

	if machine != "" {
		if err := createMachineFile(projectDir, machine); err != nil {
			return err
		}
	}

	fmt.Printf("Created Yoe-NG project at %s\n", projectDir)
	return nil
}

func createMachineFile(projectDir, name string) error {
	type machineData struct {
		Name, Arch, Console, QEMUMachine, Firmware string
	}

	data := machineData{Name: name}

	switch {
	case name == "qemu-x86_64" || name == "x86_64":
		data.Arch = "x86_64"
		data.Console = "ttyS0"
		data.QEMUMachine = "q35"
		data.Firmware = "ovmf"
	case name == "qemu-arm64" || name == "aarch64":
		data.Arch = "arm64"
		data.Console = "ttyAMA0"
		data.QEMUMachine = "virt"
		data.Firmware = "aavmf"
	case name == "qemu-riscv64" || name == "riscv64":
		data.Arch = "riscv64"
		data.Console = "ttyS0"
		data.QEMUMachine = "virt"
		data.Firmware = "opensbi"
	default:
		path := filepath.Join(projectDir, "machines", name+".toml")
		content := fmt.Sprintf("[machine]\nname = %q\narch = \"arm64\"\ndescription = \"\"\n", name)
		return os.WriteFile(path, []byte(content), 0644)
	}

	tmpl, err := template.New("machine").Parse(qemuMachineTemplate)
	if err != nil {
		return fmt.Errorf("parsing machine template: %w", err)
	}

	path := filepath.Join(projectDir, "machines", name+".toml")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating machine file: %w", err)
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}
