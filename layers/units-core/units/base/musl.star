unit(
    name = "musl",
    version = "1.2.5",
    license = "MIT",
    description = "musl libc shared library (from build container)",
    # No source — copy the musl dynamic linker/libc from the container.
    # All dynamically linked packages in the image need this.
    # The dynamic linker name varies by arch (ld-musl-<arch>.so.1).
    tasks = [
        task("build", steps=[
            """
case $ARCH in
    x86_64)  MUSL_ARCH=x86_64 ;;
    arm64)   MUSL_ARCH=aarch64 ;;
    riscv64) MUSL_ARCH=riscv64 ;;
    *)       echo "unsupported ARCH=$ARCH"; exit 1 ;;
esac
install -D /lib/ld-musl-${MUSL_ARCH}.so.1 $DESTDIR/lib/ld-musl-${MUSL_ARCH}.so.1
ln -sf ld-musl-${MUSL_ARCH}.so.1 $DESTDIR/lib/libc.musl-${MUSL_ARCH}.so.1
""",
        ]),
    ],
)
