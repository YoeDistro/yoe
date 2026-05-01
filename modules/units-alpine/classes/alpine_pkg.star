# alpine_pkg — wrap a prebuilt Alpine .apk as a yoe unit.
#
# Fetches a binary apk from the pinned Alpine release and unpacks its data
# segment into $DESTDIR. No source build, no patches — Alpine builds it for
# us. The unit's "build" is just `tar -xzpf` of the apk.
#
# ───── Alpine release coupling ────────────────────────────────────────────
#
# _ALPINE_RELEASE below MUST match the `FROM alpine:<release>` line in
# @units-core's toolchain-musl Dockerfile. The build container's libc,
# headers, and signing keys come from that Alpine release; packages this
# module fetches are ABI- and key-coupled to the same release. Mixing
# versions silently produces images that link against one libc at build
# time and a different one at install time — diagnose-once, regret-forever.
#
# When bumping _ALPINE_RELEASE: update the Dockerfile in the same commit,
# bump every alpine_pkg unit's version + sha256 to the new release, and
# rebuild the toolchain container so its baked apk-tools keyring matches.

_ALPINE_RELEASE = "v3.21"
_ALPINE_MIRROR  = "https://dl-cdn.alpinelinux.org/alpine"

# Map yoe canonical arches → Alpine arch tokens used in repo URLs.
_ARCH_MAP = {
    "x86_64":  "x86_64",
    "arm64":   "aarch64",
    "riscv64": "riscv64",
}

# .PKGINFO and .pre-/.post-install scripts live in the apk's control
# segment; .SIGN.* lives in the signature segment. None of them belong on
# the target rootfs — install scripts in particular assume Alpine's
# OpenRC/adduser layout, which yoe images don't ship. Strip them all.
_METADATA_FILES = [
    ".PKGINFO",
    ".pre-install", ".post-install",
    ".pre-deinstall", ".post-deinstall",
    ".pre-upgrade", ".post-upgrade",
    ".trigger",
]

def _install_steps(pkg_filename):
    # Build steps run with CWD set to the unit's source directory, so the
    # apk file is referenced as a path relative to '.', not via $SRCDIR
    # (which is unset at build time and would expand to empty).
    excludes = ["--exclude=" + p for p in _METADATA_FILES]
    excludes.append("--exclude=.SIGN.*")
    return [
        "mkdir -p $DESTDIR",
        "tar -xzpf ./%s -C $DESTDIR %s" % (pkg_filename, " ".join(excludes)),
    ]

def alpine_pkg(name, version, sha256,
               pkgname = None,        # apk package name if it differs from the unit name
               repo = "main",         # main | community
               runtime_deps = [],     # explicit; do not auto-pull Alpine's dep closure
               provides = [],
               replaces = [],
               license = "", description = "",
               scope = "",
               **kwargs):
    if ARCH not in _ARCH_MAP:
        fail("alpine_pkg %s: unsupported ARCH=%s (supported: %s)" %
             (name, ARCH, ", ".join(sorted(_ARCH_MAP.keys()))))
    if ARCH not in sha256:
        fail("alpine_pkg %s: sha256 has no entry for ARCH=%s" % (name, ARCH))

    apk_name = pkgname if pkgname else name
    alpine_arch = _ARCH_MAP[ARCH]
    asset = "%s-%s.apk" % (apk_name, version)
    url = "%s/%s/%s/%s/%s" % (_ALPINE_MIRROR, _ALPINE_RELEASE, repo, alpine_arch, asset)

    unit(
        name = name,
        version = version,
        source = url,
        sha256 = sha256[ARCH],
        deps = [],                      # prebuilt — no build deps
        runtime_deps = runtime_deps,
        provides = provides,
        replaces = replaces,
        license = license,
        description = description,
        scope = scope,
        # Run inside toolchain-musl just because we need GNU tar to handle
        # multi-stream gzip; nothing here actually compiles. The container
        # also pins the same Alpine release whose packages we're pulling.
        container = "toolchain-musl",
        container_arch = "target",
        sandbox = False,
        tasks = [
            task("install", steps = _install_steps(asset)),
        ],
        **kwargs
    )
