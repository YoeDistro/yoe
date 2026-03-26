image(
    name = "base-image",
    version = "1.0.0",
    description = "Minimal bootable Yoe-NG system",
    packages = [
        "busybox",
        "linux",
        "syslinux",
    ],
    hostname = "yoe",
    timezone = "UTC",
    services = [],
    partitions = [
        partition(label="rootfs", type="ext4", size="512M", root=True),
    ],
)
