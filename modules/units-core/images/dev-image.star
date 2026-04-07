load("//classes/image.star", "image")
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
    artifacts = [
        "base-files-dev", "busybox", "musl", "kmod", "util-linux",
        "linux", "network-config", "openssh", "ca-certificates", "curl",
        "simpleiot", "strace", "vim",
    ],
    hostname = "yoe-dev",
    services = ["sshd"],
)
