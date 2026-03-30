unit(
    name = "syslinux",
    version = "6.03",
    source = "https://mirrors.edge.kernel.org/pub/linux/utils/boot/syslinux/syslinux-6.03.tar.xz",
    license = "GPL-2.0",
    description = "BIOS bootloader (MBR + extlinux)",
    build = [
        # syslinux is distributed with prebuilt binaries — we just install them
        "install -D bios/mbr/mbr.bin $DESTDIR/usr/share/syslinux/mbr.bin",
        "install -D bios/mbr/gptmbr.bin $DESTDIR/usr/share/syslinux/gptmbr.bin",
        # ldlinux must be on the boot partition for extlinux to work
        "install -D bios/com32/elflink/ldlinux/ldlinux.c32 $DESTDIR/boot/extlinux/ldlinux.c32",
        "install -D bios/core/ldlinux.sys $DESTDIR/boot/extlinux/ldlinux.sys",
    ],
)
