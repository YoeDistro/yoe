package(
    name = "busybox",
    version = "1.36.1",
    source = "https://github.com/mirror/busybox.git",
    tag = "1_36_1",
    license = "GPL-2.0",
    description = "Swiss army knife of embedded Linux",
    build = [
        "make defconfig",
        # Build as static binary so it runs without shared libraries
        "sed -i 's/# CONFIG_STATIC is not set/CONFIG_STATIC=y/' .config",
        "make -j$NPROC",
        "make CONFIG_PREFIX=$DESTDIR install",
    ],
)
