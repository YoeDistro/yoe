machine(
    name = "raspberrypi4",
    arch = "arm64",
    description = "Raspberry Pi 4 Model B",
    kernel = kernel(
        unit = "linux-rpi4",
        defconfig = "bcm2711_defconfig",
        cmdline = "console=ttyS0,115200 root=/dev/mmcblk0p2 rootfstype=ext4 rootwait rw",
    ),
)
