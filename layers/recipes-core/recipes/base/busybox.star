package(
    name = "busybox",
    version = "1.36.1",
    source = "https://busybox.net/downloads/busybox-1.36.1.tar.bz2",
    sha256 = "b8cc24c9574d809e7279c3be349795c5d5ceb6fdf19ca709f80cde50e47de314",
    license = "GPL-2.0",
    description = "Swiss army knife of embedded Linux",
    build = [
        "make defconfig",
        "make -j$NPROC",
        "make CONFIG_PREFIX=$DESTDIR install",
    ],
)
