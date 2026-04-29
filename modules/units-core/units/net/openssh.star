load("//classes/autotools.star", "autotools")

autotools(
    name = "openssh",
    version = "9.9p1",
    source = "https://github.com/openssh/openssh-portable.git",
    tag = "V_9_9_P1",
    license = "BSD-2-Clause",
    description = "OpenSSH secure shell client and server",
    # Note: this unit doesn't yet ship an /etc/init.d/sshd script, so
    # there's no service to enable. Add the script and re-add
    # `services = ["sshd"]` (or `["S40sshd"]`) when implementing boot-time
    # auto-start.
    deps = ["openssl", "zlib"],
    runtime_deps = ["openssl", "zlib"],
    configure_args = [
        "--sysconfdir=/etc/ssh",
        "--without-openssl-header-check",
    ],
)
