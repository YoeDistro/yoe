# apk-tools is the Alpine package manager (apk add/del/upgrade/info). On
# yoe targets it provides on-device install, upgrade, and verification
# against the project's signed repo — the same .apk files and APKINDEX
# yoe builds at image-assembly time also work for live OTA-style updates.
#
# Build system is plain GNU Make: no autotools, no cmake. LUAAPK=no skips
# the optional lua bindings (yoe doesn't ship lua); pkg-config locates
# zlib and openssl from the per-unit sysroot.
unit(
    name = "apk-tools",
    version = "2.14.10",
    source = "https://gitlab.alpinelinux.org/alpine/apk-tools.git",
    tag = "v2.14.10",
    license = "GPL-2.0-only",
    description = "Alpine package manager — apk add/upgrade/info on target",
    deps = ["zlib", "openssl", "toolchain-musl"],
    runtime_deps = ["zlib", "openssl"],
    container = "toolchain-musl",
    container_arch = "target",
    sandbox = True,
    shell = "bash",
    tasks = [
        task("build", steps = [
            "make CC=cc LUAAPK=no -j$NPROC",
            "make CC=cc LUAAPK=no DESTDIR=$DESTDIR install",
        ]),
    ],
)
