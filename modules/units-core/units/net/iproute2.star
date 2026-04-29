# iproute2 has a hand-written configure script and Makefile-driven install
# paths, so it doesn't fit the autotools class. Build directly.
unit(
    name = "iproute2",
    version = "6.13.0",
    source = "https://github.com/iproute2/iproute2.git",
    tag = "v6.13.0",
    license = "GPL-2.0-or-later",
    description = "Full ip(8)/tc(8) suite for advanced network configuration",
    deps = ["util-linux", "toolchain-musl"],
    runtime_deps = ["util-linux"],
    container = "toolchain-musl",
    container_arch = "target",
    sandbox = True,
    shell = "bash",
    tasks = [
        task("build", steps = [
            # Disable Berkeley DB lookup (we don't ship it) and skip elf/cap
            # deps that pull in extra libraries we don't need yet.
            "./configure",
            "make CC=cc HAVE_ELF=n HAVE_CAP=n HAVE_SELINUX=n -j$NPROC " +
                "CONFDIR=/etc/iproute2",
            "make DESTDIR=$DESTDIR PREFIX=/usr SBINDIR=/sbin " +
                "CONFDIR=/etc/iproute2 install",
        ]),
    ],
)
