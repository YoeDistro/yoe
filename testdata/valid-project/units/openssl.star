unit(
    name = "openssl",
    version = "3.2.1",
    source = "https://www.openssl.org/source/openssl-3.2.1.tar.gz",
    sha256 = "abc123def456",
    deps = ["zlib"],
    runtime_deps = ["zlib"],
    tasks = [
        task("build", steps = [
            "./Configure --prefix=$PREFIX --openssldir=/etc/ssl",
            "make -j$NPROC",
            "make DESTDIR=$DESTDIR install",
        ]),
    ],
)
