image(
    name = "base-image",
    version = "1.0.0",
    description = "Minimal bootable Yoe-NG system",
    packages = [
        "busybox",
        "linux",
    ],
    hostname = "yoe",
    timezone = "UTC",
    services = [],
    partitions = [
        partition(label="boot", type="vfat", size="64M",
                  contents=["vmlinuz"]),
        partition(label="rootfs", type="ext4", size="512M", root=True),
    ],
)
