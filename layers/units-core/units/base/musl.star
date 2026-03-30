unit(
    name = "musl",
    version = "1.2.5",
    license = "MIT",
    description = "musl libc shared library (from build container)",
    # No source — copy the musl dynamic linker/libc from the container.
    # All dynamically linked packages in the image need this.
    build = [
        "install -D /lib/ld-musl-x86_64.so.1 $DESTDIR/lib/ld-musl-x86_64.so.1",
        "ln -sf ld-musl-x86_64.so.1 $DESTDIR/lib/libc.musl-x86_64.so.1",
    ],
)
