image(
    name = "base-image",
    version = "1.0.0",
    scope = "machine",
    description = "Minimal bootable Yoe-NG system",
    artifacts = [
        "base-files",
        "busybox",
        "linux",
    ] + (["syslinux"] if ARCH == "x86_64" else []),
    hostname = "yoe",
    timezone = "UTC",
    services = [],
    partitions = [
        partition(label="rootfs", type="ext4", size="128M", root=True),
    ],
)
