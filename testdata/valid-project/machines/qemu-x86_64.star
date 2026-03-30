machine(
    name = "qemu-x86_64",
    arch = "x86_64",
    kernel = kernel(unit = "linux-qemu", cmdline = "console=ttyS0 root=/dev/vda2 rw"),
    qemu = qemu_config(machine = "q35", cpu = "host", memory = "1G", firmware = "ovmf", display = "none"),
)
