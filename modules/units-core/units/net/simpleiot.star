load("//classes/go.star", "go_binary")

go_binary(
    name = "simpleiot",
    version = "0.18.5",
    source = "https://github.com/simpleiot/simpleiot.git",
    tag = "v0.18.5",
    go_package = "./cmd/siot",
    license = "Apache-2.0",
    description = "IoT application for sensor data, telemetry, configuration, and device management",
    services = ["simpleiot"],
    tasks = [
        task("build", steps=[
            'case "$ARCH" in x86_64) goarch=amd64;; aarch64) goarch=arm64;; armv7l) goarch=arm;; riscv64) goarch=riscv64;; *) echo "unsupported ARCH=$ARCH" >&2; exit 1;; esac'
            + " && export PATH=/usr/local/go/bin:$PATH"
            + " && CGO_ENABLED=0 GOOS=linux GOARCH=$goarch"
            + " go build -o $DESTDIR$PREFIX/bin/siot ./cmd/siot",
        ]),
        task("init-script", steps=[
            "mkdir -p $DESTDIR/etc/init.d",
            """cat > $DESTDIR/etc/init.d/simpleiot << 'INIT'
#!/bin/sh
case "$1" in
    start) /usr/bin/siot &;;
    stop) killall siot;;
esac
INIT""",
            "chmod +x $DESTDIR/etc/init.d/simpleiot",
        ]),
    ],
)
