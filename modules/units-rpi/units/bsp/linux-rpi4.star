unit(
    name = "linux-rpi4",
    version = "6.12",
    scope = "machine",
    source = "https://github.com/raspberrypi/linux.git",
    branch = "rpi-6.12.y",
    license = "GPL-2.0",
    description = "Raspberry Pi 4 kernel (BCM2711)",
    tasks = [
        task("build", steps=[
            "make ARCH=arm64 bcm2711_defconfig",
            "make ARCH=arm64 -j$NPROC Image dtbs",
            # Install kernel as kernel8.img (RPi4 64-bit naming convention)
            "install -D arch/arm64/boot/Image $DESTDIR/boot/kernel8.img",
            # Install device trees
            "install -D arch/arm64/boot/dts/broadcom/bcm2711-rpi-4-b.dtb $DESTDIR/boot/bcm2711-rpi-4-b.dtb",
            "install -D arch/arm64/boot/dts/broadcom/bcm2711-rpi-400.dtb $DESTDIR/boot/bcm2711-rpi-400.dtb",
            "install -D arch/arm64/boot/dts/broadcom/bcm2711-rpi-cm4.dtb $DESTDIR/boot/bcm2711-rpi-cm4.dtb",
            # Install overlays directory
            "mkdir -p $DESTDIR/boot/overlays",
            "cp arch/arm64/boot/dts/overlays/*.dtbo $DESTDIR/boot/overlays/ 2>/dev/null || true",
        ]),
    ],
)
