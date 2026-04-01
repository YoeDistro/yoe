unit(
    name = "linux-rpi5",
    version = "6.12",
    scope = "machine",
    source = "https://github.com/raspberrypi/linux.git",
    branch = "rpi-6.12.y",
    license = "GPL-2.0",
    description = "Raspberry Pi 5 kernel (BCM2712)",
    tasks = [
        task("build", steps=[
            "make ARCH=arm64 bcm2712_defconfig",
            "make ARCH=arm64 -j$NPROC Image dtbs",
            # Install kernel as kernel_2712.img (RPi5 naming convention)
            "install -D arch/arm64/boot/Image $DESTDIR/boot/kernel_2712.img",
            # Install device trees
            "install -D arch/arm64/boot/dts/broadcom/bcm2712-rpi-5-b.dtb $DESTDIR/boot/bcm2712-rpi-5-b.dtb",
            # Install overlays directory
            "mkdir -p $DESTDIR/boot/overlays",
            "cp arch/arm64/boot/dts/overlays/*.dtbo $DESTDIR/boot/overlays/ 2>/dev/null || true",
        ]),
    ],
)
