load("//classes/users.star", "user", "users_commands")

def base_files(name = "base-files", users = None):
    """Creates a base filesystem skeleton unit with the given users.

    Override this in your image to add users:
        load("//units/base/base-files.star", "base_files")
        load("//classes/users.star", "user")
        base_files(name = "base-files-dev", users = [
            user(name = "root", uid = 0, gid = 0, home = "/root"),
            user(name = "myuser", uid = 1000, gid = 1000, password = "secret"),
        ])
    """
    if not users:
        users = [
            user(name = "root", uid = 0, gid = 0, home = "/root"),
        ]

    # openssl is needed at build time if any user has a password to hash
    deps = []
    for u in users:
        if u["password"]:
            deps.append("openssl")
            break

    unit(
        name = name,
        version = "1.0.0",
        license = "MIT",
        description = "Base filesystem skeleton: users, groups, dirs, inittab, boot config",
        deps = deps,
        build = [
            # Essential directories
            "mkdir -p $DESTDIR/etc $DESTDIR/root $DESTDIR/proc $DESTDIR/sys"
            + " $DESTDIR/dev $DESTDIR/tmp $DESTDIR/run",
        ] + users_commands(users) + [
            # Busybox inittab: mount filesystems, getty on serial console.
            # Console device varies by arch (ttyS0 on x86, ttyAMA0 on arm64).
            """
case $ARCH in
    arm64)   CONSOLE=ttyAMA0 ;;
    *)       CONSOLE=ttyS0 ;;
esac
cat > $DESTDIR/etc/inittab << INITTAB
::sysinit:/bin/mount -t proc proc /proc
::sysinit:/bin/mount -t sysfs sys /sys
::sysinit:/bin/hostname -F /etc/hostname
::sysinit:/etc/init.d/rcS
${CONSOLE}::respawn:/sbin/getty -L ${CONSOLE} 115200 vt100
::ctrlaltdel:/sbin/reboot
::shutdown:/bin/umount -a -r
INITTAB
""",

            # rcS script — runs all init scripts in /etc/init.d/
            "mkdir -p $DESTDIR/etc/init.d",
            "cat > $DESTDIR/etc/init.d/rcS << 'RCS'\n"
            + "#!/bin/sh\n"
            + "for s in /etc/init.d/S*; do\n"
            + "    [ -x \"$s\" ] && \"$s\" start\n"
            + "done\n"
            + "RCS",
            "chmod +x $DESTDIR/etc/init.d/rcS",

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

# Default: base-files with just root (blank password)
base_files()
