package(
    name = "busybox",
    version = "1.36.1",
    source = "https://github.com/mirror/busybox.git",
    tag = "1_36_1",
    license = "GPL-2.0",
    description = "Swiss army knife of embedded Linux",
    build = [
        "make defconfig",
        "make -j$NPROC",
        "make CONFIG_PREFIX=$DESTDIR install",
    ],
)
