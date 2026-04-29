def image(name, artifacts=[], hostname="", timezone="", locale="",
          partitions=[], scope="machine",
          container="toolchain-musl", container_arch="target", deps=[], **kwargs):
    """Create a bootable disk image from packages."""
    # Merge machine packages
    all_artifacts = list(artifacts) + list(MACHINE_CONFIG.packages)

    # Resolve provides (e.g., "linux" → "linux-rpi4")
    resolved = []
    for a in all_artifacts:
        r = PROVIDES.get(a, None)
        resolved.append(r if r != None else a)

    # Resolve transitive runtime dependencies
    resolved = _resolve_runtime_deps(resolved)

    # Use machine partitions if image doesn't specify its own
    all_partitions = partitions if partitions else list(MACHINE_CONFIG.partitions)

    # Merge class deps with user deps
    all_deps = list(deps)
    if container and container not in all_deps:
        all_deps.append(container)

    unit(
        name = name,
        scope = scope,
        unit_class = "image",
        artifacts = resolved,
        partitions = all_partitions,
        container = container,
        container_arch = container_arch,
        sandbox = True,
        shell = "bash",
        deps = all_deps,
        tasks = [
            task("rootfs", fn=lambda: _assemble_rootfs(resolved, hostname, timezone, locale)),
            task("disk", fn=lambda: _create_disk_image(name, all_partitions)),
        ],
        **kwargs,
    )

def _assemble_rootfs(packages, hostname, timezone, locale):
    """Install packages into the rootfs using apk-tools.

    apk handles dependency resolution from APKINDEX, enforces file-conflict
    detection, and populates /lib/apk/db/installed automatically. The
    `packages` list still includes transitive runtime deps from
    `_resolve_runtime_deps` so the build-time DAG schedules everything,
    but apk will re-resolve install order itself.

    Flags:
      --root           — destination rootfs
      --initdb         — create /lib/apk/db on a fresh rootfs
      --allow-untrusted — until phase 3 of the apk-compat plan adds signing
      --no-network     — never reach the public Alpine mirrors
      --no-cache       — keep /etc/apk/cache out of the rootfs
      --no-scripts     — don't try to run pre/post-install scripts during
                          assembly; the rootfs has no /bin/sh yet, and
                          yoe-built apks don't ship scripts today anyway
      -X $REPO         — yoe's local Alpine-layout repo
      --force-overwrite — tolerate intentional shadow files (busybox vs
                          iproute2, util-linux, procps-ng, etc.) until
                          phase 5 ships `replaces:` annotations
    """
    run("mkdir -p $DESTDIR/rootfs")

    pkg_args = " ".join(packages)
    run("apk add " +
        "--root $DESTDIR/rootfs " +
        "--initdb " +
        "--allow-untrusted " +
        "--no-network " +
        "--no-cache " +
        "--no-scripts " +
        "--force-overwrite " +
        "-X $REPO " +
        pkg_args)

    if hostname:
        run("mkdir -p $DESTDIR/rootfs/etc")
        run("echo %s > $DESTDIR/rootfs/etc/hostname" % hostname)

    if timezone:
        run("mkdir -p $DESTDIR/rootfs/etc")
        run("echo %s > $DESTDIR/rootfs/etc/timezone" % timezone)
    # Note: init.d service symlinks are baked into each apk's data tar at
    # package-time (see internal/artifact/apk.go's materializeServiceSymlinks),
    # so apk add — image-time or on-target — produces the same rootfs. yoe
    # does not patch the rootfs after install.

def _create_disk_image(name, partitions):
    if not partitions:
        return

    total_mb = 1
    for p in partitions:
        total_mb += _parse_size_mb(p.size)

    img = "$DESTDIR/%s.img" % name
    run("dd if=/dev/zero of=%s bs=1M count=0 seek=%d" % (img, total_mb))

    sfdisk_lines = "label: dos\\n"
    for i, p in enumerate(partitions):
        size_mb = _parse_size_mb(p.size)
        ptype = "c" if p.type == "vfat" else "83"
        # Only specify size for non-last partitions; last gets remaining space
        size_spec = "size=%dMiB, " % size_mb if i < len(partitions) - 1 else ""
        bootable = ", bootable" if p.root else ""
        sfdisk_lines += "%stype=%s%s\\n" % (size_spec, ptype, bootable)

    run("printf '%s' | sfdisk %s" % (sfdisk_lines, img))

    # Rootfs was assembled as the host build user (docker --user uid:gid), so
    # every file under $DESTDIR/rootfs is owned by that uid. mkfs.ext4 -d copies
    # ownership into the filesystem verbatim, so the booted system would see
    # files owned by whatever host user ran the build. Chown to root before
    # packing, and chown the destdir back at the end so the next build's
    # os.RemoveAll() on the host can clean it up.
    run("chown -R 0:0 $DESTDIR/rootfs", privileged = True)

    offset = 1
    for p in partitions:
        size_mb = _parse_size_mb(p.size)
        part_img = img + "." + p.label + ".part"
        run("dd if=/dev/zero of=%s bs=1M count=0 seek=%d" % (part_img, size_mb))

        if p.type == "vfat":
            run("mkfs.vfat -n %s %s" % (p.label.upper(), part_img))
            # Copy boot files from rootfs (root-owned; mcopy needs read access).
            run("mcopy -sQi %s $DESTDIR/rootfs/boot/* ::/ 2>/dev/null || true" % part_img, privileged = True)
        elif p.type == "ext4":
            # Disable ext4 features that syslinux 6.03 can't read (x86 only)
            ext4_opts = "-O ^64bit,^metadata_csum,^extent " if ARCH == "x86_64" else ""
            run("mkfs.ext4 %s-d $DESTDIR/rootfs -L %s %s %dM" % (ext4_opts, p.label, part_img, size_mb),
                privileged = True)

        run("dd if=%s of=%s bs=1M seek=%d conv=notrunc" % (part_img, img, offset))
        run("rm -f %s" % part_img)
        offset += size_mb

    # Install bootloader (x86 syslinux)
    if ARCH == "x86_64":
        _install_syslinux(img, partitions)

    # Restore destdir ownership to the host build user. The chown -R above,
    # plus any root-owned files the privileged mkfs/mcopy/syslinux steps left
    # behind, would otherwise block the next build's os.RemoveAll on the host.
    run("chown -R $(stat -c %u:%g /project) $DESTDIR", privileged = True)

def _install_syslinux(img, partitions):
    """Install syslinux MBR boot code and extlinux on an x86 disk image."""
    # Write MBR boot code (first 440 bytes of mbr.bin)
    run("dd if=$DESTDIR/rootfs/usr/share/syslinux/mbr.bin of=%s bs=440 count=1 conv=notrunc" % img)

    # Find the root partition offset and size
    offset_mb = 1  # MBR overhead
    root_size_mb = 0
    for p in partitions:
        size = _parse_size_mb(p.size)
        if p.root:
            root_size_mb = size
            break
        offset_mb += size

    if root_size_mb == 0:
        return

    offset_bytes = offset_mb * 1024 * 1024
    size_bytes = root_size_mb * 1024 * 1024

    # Run extlinux --install via losetup with explicit offset (not -P which
    # requires partition device nodes). Needs privileged=True for losetup/mount.
    # Docker's --privileged does not populate /dev/loop*, so losetup --find
    # allocates a loop number via /dev/loop-control but then fails to open the
    # missing device node. Pre-create /dev/loop0..31 via mknod before losetup.
    run("""
set -e
for i in $(seq 0 31); do
    [ -b /dev/loop$i ] || mknod /dev/loop$i b 7 $i
done
LOOP=$(losetup --find --show --offset %d --sizelimit %d %s)
trap 'umount /mnt/extlinux 2>/dev/null; losetup -d $LOOP 2>/dev/null' EXIT
mkdir -p /mnt/extlinux
mount -t ext4 $LOOP /mnt/extlinux
extlinux --install /mnt/extlinux/boot/extlinux
""" % (offset_bytes, size_bytes, img), privileged=True)

def _resolve_runtime_deps(packages):
    """Expand a package list to include all transitive runtime dependencies.
    Starlark has no recursion or while loops, so we use iterative BFS
    with a for loop over a generous upper bound.
    """
    # BFS: discover all transitive runtime deps
    seen = {}
    queue = list(packages)
    for _i in range(1000):  # upper bound on iterations
        if not queue:
            break
        name = queue[0]
        queue = queue[1:]
        if name in seen:
            continue
        seen[name] = True
        deps = RUNTIME_DEPS.get(name, None)
        if deps != None:
            for dep in deps:
                resolved = PROVIDES.get(dep, None)
                d = resolved if resolved != None else dep
                if d not in seen:
                    queue = queue + [d]

    # Topological sort: emit packages whose deps are all emitted
    remaining = list(seen.keys())
    ordered = []
    emitted = {}
    for _round in range(len(remaining) + 1):
        next_remaining = []
        for name in remaining:
            deps = RUNTIME_DEPS.get(name, None)
            ready = True
            if deps != None:
                for dep in deps:
                    resolved = PROVIDES.get(dep, None)
                    d = resolved if resolved != None else dep
                    if d in seen and d not in emitted:
                        ready = False
                        break
            if ready:
                ordered.append(name)
                emitted[name] = True
            else:
                next_remaining.append(name)
        remaining = next_remaining
        if not remaining:
            break
    # Append any remaining (circular deps)
    for name in remaining:
        ordered.append(name)
    return ordered

def _parse_size_mb(size_str, default=256):
    """Parse a size string like '64M', '1G', or 'fill' into megabytes."""
    s = str(size_str)
    if s == "fill" or s == "":
        return default
    if s.endswith("M"):
        return int(s[:-1])
    if s.endswith("G"):
        return int(s[:-1]) * 1024
    return int(s)
