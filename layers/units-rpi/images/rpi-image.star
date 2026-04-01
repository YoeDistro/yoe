# Minimal bootable Raspberry Pi image.
# Select kernel and config based on the target machine.

_rpi4_units = ["linux-rpi4", "rpi4-config"]
_rpi5_units = ["linux-rpi5", "rpi5-config"]

_machine_units = (
    _rpi4_units if MACHINE == "raspberrypi4"
    else _rpi5_units if MACHINE == "raspberrypi5"
    else []
)

image(
    name = "rpi-image",
    version = "1.0.0",
    scope = "machine",
    description = "Minimal bootable Raspberry Pi image",
    artifacts = [
        "base-files",
        "busybox",
        "musl",
    ] + _machine_units + [
        "rpi-firmware",
    ],
    hostname = "yoe-rpi",
    timezone = "UTC",
    partitions = [
        partition(label="boot", type="vfat", size="64M", contents=["*"]),
        partition(label="rootfs", type="ext4", size="256M", root=True),
    ],
)
