image(
    name = "base-image",
    version = "1.0.0",
    description = "Minimal bootable Yoe-NG system",
    packages = [
        "base-files",
        "busybox",
        "linux",
        "syslinux",
    ],
    hostname = "yoe",
    timezone = "UTC",
    services = [],
    partitions = [
        partition(label="rootfs", type="ext4", size="50M", root=True),
    ],
)
