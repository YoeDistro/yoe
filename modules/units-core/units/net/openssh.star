load("//classes/autotools.star", "autotools")

autotools(
    name = "openssh",
    version = "9.9p1",
    source = "https://github.com/openssh/openssh-portable.git",
    tag = "V_9_9_P1",
    license = "BSD-2-Clause",
    description = "OpenSSH secure shell client and server",
    services = ["sshd"],
    deps = ["openssl", "zlib"],
    runtime_deps = ["openssl", "zlib"],
    configure_args = [
        "--sysconfdir=/etc/ssh",
        "--without-openssl-header-check",
    ],
)
