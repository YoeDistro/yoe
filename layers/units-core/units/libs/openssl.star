unit(
    name = "openssl",
    version = "3.4.1",
    source = "https://github.com/openssl/openssl.git",
    tag = "openssl-3.4.1",
    license = "Apache-2.0",
    description = "TLS/SSL and crypto library",
    deps = ["zlib"],
    runtime_deps = ["zlib"],
    build = [
        "./Configure --prefix=$PREFIX --libdir=lib --openssldir=/etc/ssl shared zlib",
        "make -j$NPROC",
        "make DESTDIR=$DESTDIR install_sw install_ssldirs",
    ],
)
