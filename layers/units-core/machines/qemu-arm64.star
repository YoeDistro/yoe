machine(
    name = "qemu-arm64",
    arch = "arm64",
    description = "QEMU ARM64 virtual machine",
    kernel = kernel(
        unit = "linux",
        provides = "linux",
        defconfig = "defconfig",
        cmdline = "console=ttyAMA0 root=/dev/vda1 rw",
    ),
    partitions = [
        partition(label = "rootfs", type = "ext4", size = "128M", root = True),
    ],
    qemu = qemu_config(
        machine = "virt",
        cpu = "host",
        memory = "1G",
        display = "none",
    ),
)
