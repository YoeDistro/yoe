unit(
    name = "openssh",
    version = "9.6p1",
    description = "OpenSSH client and server",
    license = "BSD",
    source = "https://cdn.openbsd.org/pub/OpenBSD/OpenSSH/portable/openssh-9.6p1.tar.gz",
    sha256 = "aaaa1111bbbb2222",
    deps = ["zlib", "openssl", "toolchain-musl"],
    runtime_deps = ["zlib", "openssl"],
    container = "toolchain-musl",
    container_arch = "target",
    tasks = [
        task("build", steps = [
            "./configure --prefix=$PREFIX --sysconfdir=/etc/ssh",
            "make -j$NPROC",
            "make DESTDIR=$DESTDIR install",
        ]),
    ],
    services = ["sshd"],
    conffiles = ["/etc/ssh/sshd_config"],
)
