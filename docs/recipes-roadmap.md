# Recipes Roadmap

Recipes needed for a complete base Linux system. Existing recipes can be found
via `yoe list` or by browsing `layers/recipes-core/recipes/`.

## Needed Recipes

### High Priority — Required for a Functional System

- [ ] `util-linux` — mount, fdisk, blkid, lsblk, login, agetty; essential for
      booting and managing disks
- [ ] `kmod` — modprobe, insmod, depmod; kernel module loading
- [ ] `eudev` — device manager for dynamic /dev; alternative is busybox mdev but
      eudev is more capable
- [ ] `e2fsprogs` — mkfs.ext4, fsck.ext4; required for ext4 filesystem
      maintenance

### Medium Priority — Required for Networking and Security

- [ ] `ca-certificates` — root CA bundle for TLS verification; without this,
      curl and openssh cannot verify peers
- [ ] `iproute2` — `ip` command for network configuration; busybox version is
      limited
- [ ] `dhcpcd` — DHCP client; busybox udhcpc works but dhcpcd handles more edge
      cases
- [ ] `iptables` or `nftables` — firewall; important for any networked device

### Low Priority — Nice to Have

- [ ] `dbus` — IPC message bus; dependency for many higher-level services
- [ ] `bash` — full shell if busybox ash is insufficient
- [ ] `less` — pager with more features than busybox less
- [ ] `file` — file type identification
- [ ] `procps` — ps, top, free, vmstat; more capable than busybox versions
- [ ] `htop` — interactive process viewer
- [ ] `dropbear` — lightweight SSH alternative to openssh for constrained devices

## Notes

- Every dependency must be built from source as a recipe. Never install packages
  in the container Dockerfile.
- For non-essential build features (docs, man pages), prefer disabling via
  configure flags over adding build tool recipes.
- Check Alpine, Yocto, and Buildroot packaging before writing new recipes — they
  are good references for configure flags, deps, and patches.
