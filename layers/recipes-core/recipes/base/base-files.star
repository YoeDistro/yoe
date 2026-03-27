package(
    name = "base-files",
    version = "1.0.0",
    license = "MIT",
    description = "Base filesystem skeleton: users, groups, essential dirs",
    build = [
        # /etc/passwd, /etc/group, /etc/shadow — root user with blank password
        "mkdir -p $DESTDIR/etc $DESTDIR/root",
        "echo 'root:x:0:0:root:/root:/bin/sh' > $DESTDIR/etc/passwd",
        "echo 'root:x:0:' > $DESTDIR/etc/group",
        "echo 'root::0:0:99999:7:::' > $DESTDIR/etc/shadow",
        "chmod 0600 $DESTDIR/etc/shadow",
        "chmod 0700 $DESTDIR/root",
    ],
)
