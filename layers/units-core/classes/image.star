def image(name, artifacts=[], hostname="", timezone="", locale="",
          services=[], partitions=[], scope="machine", **kwargs):
    """Create a bootable disk image from packages."""
    # Merge machine packages
    all_artifacts = list(artifacts) + list(MACHINE_CONFIG.packages)

    # Resolve provides (e.g., "linux" → "linux-rpi4")
    resolved = []
    for a in all_artifacts:
        r = PROVIDES.get(a, None)
        if r != None:
            resolved.append(r)
        else:
            resolved.append(a)

    # Use machine partitions if image doesn't specify its own
    all_partitions = partitions if partitions else list(MACHINE_CONFIG.partitions)

    unit(
        name = name,
        scope = scope,
        artifacts = resolved,
        partitions = all_partitions,
        tasks = [
            task("rootfs", fn=lambda: _assemble_rootfs(resolved, hostname, timezone, locale, services)),
            task("disk", fn=lambda: _create_disk_image(name, all_partitions)),
        ],
        **kwargs,
    )

def _assemble_rootfs(packages, hostname, timezone, locale, services):
    run("mkdir -p $DESTDIR/rootfs")
    for pkg in packages:
        result = run("ls $REPO/%s-*.apk 2>/dev/null | head -1" % pkg, check=False)
        if result.exit_code != 0 or str(result.stdout).strip() == "":
            run("echo 'warning: package %s not found, skipping' >&2" % pkg)
            continue
        apk = str(result.stdout).strip()
        run("tar xzf %s -C $DESTDIR/rootfs --exclude=.PKGINFO" % apk)

    if hostname:
        run("mkdir -p $DESTDIR/rootfs/etc")
        run("echo %s > $DESTDIR/rootfs/etc/hostname" % hostname)

    if timezone:
        run("mkdir -p $DESTDIR/rootfs/etc")
        run("echo %s > $DESTDIR/rootfs/etc/timezone" % timezone)

    for svc in services:
        run("test -f $DESTDIR/rootfs/etc/init.d/%s && ln -sf ../init.d/%s $DESTDIR/rootfs/etc/init.d/S50%s || true" % (svc, svc, svc))

def _create_disk_image(name, partitions):
    if not partitions:
        return

    total_mb = 1
    for p in partitions:
        total_mb += _parse_size_mb(p.size)

    img = "$DESTDIR/%s.img" % name
    run("dd if=/dev/zero of=%s bs=1M count=0 seek=%d" % (img, total_mb))

    sfdisk_lines = "label: dos\\n"
    for p in partitions:
        size_mb = _parse_size_mb(p.size)
        ptype = "c" if p.type == "vfat" else "83"
        sfdisk_lines += "- %dM %s\\n" % (size_mb, ptype)

    run("printf '%s' | sfdisk %s" % (sfdisk_lines, img))

    offset = 1
    for p in partitions:
        size_mb = _parse_size_mb(p.size)
        part_img = img + "." + p.label + ".part"
        run("dd if=/dev/zero of=%s bs=1M count=0 seek=%d" % (part_img, size_mb))

        if p.type == "vfat":
            run("mkfs.vfat -n %s %s" % (p.label.upper(), part_img))
            # Copy boot files from rootfs
            run("mcopy -sQi %s $DESTDIR/rootfs/boot/* ::/ 2>/dev/null || true" % part_img)
        elif p.type == "ext4":
            run("mkfs.ext4 -d $DESTDIR/rootfs -L %s %s %dM" % (p.label, part_img, size_mb))

        run("dd if=%s of=%s bs=1M seek=%d conv=notrunc" % (part_img, img, offset))
        run("rm -f %s" % part_img)
        offset += size_mb

def _parse_size_mb(size_str):
    s = str(size_str)
    if s.endswith("M"):
        return int(s[:-1])
    if s.endswith("G"):
        return int(s[:-1]) * 1024
    return int(s)
