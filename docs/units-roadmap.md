# Units Roadmap

Units needed for a complete base Linux system. Existing units can be found via
`yoe list` or by browsing `modules/units-core/units/`.

## Needed Units

### Medium Priority — Networking and Security

- [ ] `nftables` — modern firewall (preferred over legacy iptables). Requires
      new dep units `libmnl`, `libnftnl`, and `gmp` before it can be written.

### Low Priority — Nice to Have

- [ ] `dbus` — IPC message bus; dependency for many higher-level services. Pulls
      in expat (already present) plus a service supervisor — non-trivial, defer
      until a unit needs it.

## Implemented

The following base-system units are in `modules/units-core/units/`:

### Base

- `util-linux` — mount, fdisk, blkid, lsblk, agetty, etc.
- `kmod` — modprobe, insmod, depmod
- `ca-certificates` — Mozilla CA bundle for TLS verification
- `e2fsprogs` — mkfs.ext4, fsck.ext4, tune2fs, e2fsck
- `eudev` — dynamic /dev manager (udevd, udevadm)
- `bash` — full GNU shell
- `less` — pager
- `file` — file type identification
- `busybox`, `coreutils`, `musl`, `linux`, `base-files`

### Networking

- `openssh`, `curl`, `mdnsd`, `ntp-client`, `network-config`, `simpleiot`
- `iproute2` — full ip(8)/tc(8)
- `dhcpcd` — DHCPv4/DHCPv6 client

### Libraries

- `openssl`, `zlib`, `zstd`, `xz`, `ncurses`, `readline`, `expat`, `gettext`,
  `libffi`

### Debug / Tools

- `strace`, `vim`, `procps-ng` (ps, top, free, vmstat), `htop`

### Bootloaders / Build tools

- `syslinux`, `meson`, `samurai`, `gawk`

## Notes

- Every dependency must be built from source as a unit. Never install packages
  in the container Dockerfile.
- For non-essential build features (docs, man pages), prefer disabling via
  configure flags over adding build tool units.
- Check Alpine, Yocto, and Buildroot packaging before writing new units — they
  are good references for configure flags, deps, and patches.
- `dropbear` is intentionally not on the roadmap; the project standardizes on
  `openssh`.
