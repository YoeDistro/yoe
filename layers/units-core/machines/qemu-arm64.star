machine(
    name = "qemu-arm64",
    arch = "arm64",
    description = "QEMU ARM64 virtual machine",
    kernel = kernel(
        unit = "linux",
        defconfig = "defconfig",
        cmdline = "console=ttyAMA0 root=/dev/vda2 rw",
    ),
    qemu = qemu_config(
        machine = "virt",
        cpu = "host",
        memory = "1G",
        display = "none",
    ),
)
