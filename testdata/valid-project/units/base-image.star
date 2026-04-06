image(
    name = "base-image",
    version = "1.0.0",
    description = "Minimal bootable system",
    artifacts = ["openssh", "myapp"],
    container = "toolchain-musl",
    container_arch = "target",
    hostname = "yoe",
    timezone = "UTC",
    services = ["sshd", "myapp"],
    partitions = [
        partition(label="boot", type="vfat", size="64M"),
        partition(label="rootfs", type="ext4", size="fill", root=True),
    ],
)
