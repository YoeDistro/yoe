load("//classes/users.star", "user")
load("//units/base/base-files.star", "base_files")

base_files(
    name = "base-files-dev",
    users = [
        user(name = "root", uid = 0, gid = 0, home = "/root"),
        user(name = "user", uid = 1000, gid = 1000, password = "password"),
    ],
)

image(
    name = "dev-image",
    version = "1.0.0",
    scope = "machine",
    description = "Development image with networking, SSH, and debug tools",
    artifacts = [
        # Base
        "base-files-dev",
        "busybox",
        "musl",
        "kmod",
        "util-linux",
        "linux",
        "syslinux",
        # Networking
        "network-config",
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
        partition(label = "rootfs", type = "ext4", size = "256M", root = True),
    ],
)
