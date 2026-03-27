package(
    name = "base-files",
    version = "1.0.0",
    license = "MIT",
    description = "Base filesystem skeleton: users, groups, dirs, inittab, boot config",
    build = [
        # Essential directories
        "mkdir -p $DESTDIR/etc $DESTDIR/root $DESTDIR/proc $DESTDIR/sys"
        + " $DESTDIR/dev $DESTDIR/tmp $DESTDIR/run",

        # Root user with blank password
        "echo 'root:x:0:0:root:/root:/bin/sh' > $DESTDIR/etc/passwd",
        "echo 'root:x:0:' > $DESTDIR/etc/group",
        "echo 'root::0:0:99999:7:::' > $DESTDIR/etc/shadow",
        "chmod 0600 $DESTDIR/etc/shadow",
        "chmod 0700 $DESTDIR/root",

        # Busybox inittab: mount filesystems, getty on serial console
        "cat > $DESTDIR/etc/inittab << 'INITTAB'\n"
        + "::sysinit:/bin/mount -t proc proc /proc\n"
        + "::sysinit:/bin/mount -t sysfs sys /sys\n"
        + "::sysinit:/bin/hostname -F /etc/hostname\n"
        + "ttyS0::respawn:/sbin/getty -L ttyS0 115200 vt100\n"
        + "::ctrlaltdel:/sbin/reboot\n"
        + "::shutdown:/bin/umount -a -r\n"
        + "INITTAB",

        # Boot configuration (extlinux for QEMU serial console)
        "mkdir -p $DESTDIR/boot/extlinux",
        "cat > $DESTDIR/boot/extlinux/extlinux.conf << 'EXTLINUX'\n"
        + "DEFAULT yoe\n"
        + "LABEL yoe\n"
        + "    LINUX /boot/vmlinuz\n"
        + "    APPEND console=ttyS0 root=/dev/vda1 rw devtmpfs.mount=1\n"
        + "EXTLINUX",
    ],
)
