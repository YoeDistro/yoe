unit(
    name = "network-config",
    version = "1.0.0",
    license = "MIT",
    description = "DHCP networking via busybox udhcpc on eth0",
    services = ["S10network"],
    runtime_deps = ["busybox"],
    deps = ["toolchain-musl"],
    container = "toolchain-musl",
    container_arch = "target",
    tasks = [
        task("build", steps=[
            # udhcpc default script — busybox udhcpc calls this to apply leases
            "mkdir -p $DESTDIR/usr/share/udhcpc",
            """cat > $DESTDIR/usr/share/udhcpc/default.script << 'SCRIPT'
#!/bin/sh
case "$1" in
    bound|renew)
        ip addr flush dev "$interface"
        ip addr add "$ip/${mask:-24}" dev "$interface"
        [ -n "$router" ] && ip route add default via "$router"
        [ -n "$dns" ] && {
            : > /etc/resolv.conf
            for d in $dns; do
                echo "nameserver $d" >> /etc/resolv.conf
            done
        }
        ;;
    deconfig)
        ip addr flush dev "$interface"
        ;;
esac
SCRIPT""",
            "chmod +x $DESTDIR/usr/share/udhcpc/default.script",

            # Init script to bring up networking
            "mkdir -p $DESTDIR/etc/init.d",
            """cat > $DESTDIR/etc/init.d/S10network << 'INIT'
#!/bin/sh
ip link set lo up
ip link set eth0 up
udhcpc -i eth0 -s /usr/share/udhcpc/default.script -q
INIT""",
            "chmod +x $DESTDIR/etc/init.d/S10network",
        ]),
    ],
)
