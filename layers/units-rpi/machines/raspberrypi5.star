machine(
    name = "raspberrypi5",
    arch = "arm64",
    description = "Raspberry Pi 5",
    kernel = kernel(
        unit = "linux-rpi5",
        defconfig = "bcm2712_defconfig",
        cmdline = "console=ttyAMA10,115200 root=/dev/mmcblk0p2 rootfstype=ext4 rootwait rw",
    ),
)
