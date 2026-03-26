package(
    name = "linux",
    version = "6.6.87",
    source = "https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.6.87.tar.xz",
    sha256 = "89e0e40d3e8b7cae8b3e3b0e5fa7e84c7d2117aae5de83fc3eb79e75109a96ec",
    license = "GPL-2.0",
    description = "Linux kernel",
    build = [
        "make x86_64_defconfig",
        "make -j$NPROC bzImage",
        "install -D arch/x86/boot/bzImage $DESTDIR/boot/vmlinuz",
    ],
)
