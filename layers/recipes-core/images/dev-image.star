image(
    name = "dev-image",
    version = "1.0.0",
    description = "Development image with networking, SSH, and debug tools",
    packages = [
        # Base
        "busybox",
        "linux",
        "syslinux",
        # Networking
        "openssh",
        "curl",
        # Debug
        "strace",
        "vim",
    ],
    hostname = "yoe-dev",
    timezone = "UTC",
    services = ["sshd"],
    partitions = [
        partition(label="rootfs", type="ext4", size="1G", root=True),
    ],
)
