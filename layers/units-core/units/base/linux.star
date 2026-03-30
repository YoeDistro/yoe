unit(
    name = "linux",
    version = "6.6.87",
    source = "https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git",
    tag = "v6.6.87",
    license = "GPL-2.0",
    description = "Linux kernel",
    build = [
        "make x86_64_defconfig",
        "make -j$NPROC bzImage",
        "install -D arch/x86/boot/bzImage $DESTDIR/boot/vmlinuz",
    ],
)
