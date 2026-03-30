machine(
    name = "qemu-x86_64",
    arch = "x86_64",
    description = "QEMU x86_64 virtual machine (KVM)",
    kernel = kernel(
        unit = "linux",
        defconfig = "x86_64_defconfig",
        cmdline = "console=ttyS0 root=/dev/vda2 rw",
    ),
    qemu = qemu_config(
        machine = "q35",
        cpu = "host",
        memory = "1G",
        firmware = "seabios",
        display = "none",
    ),
)
