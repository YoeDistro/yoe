unit(
    name = "myapp",
    version = "1.2.3",
    description = "Edge data collection service",
    license = "Apache-2.0",
    source = "https://github.com/example/myapp.git",
    tag = "v1.2.3",
    container = "toolchain-musl",
    container_arch = "target",
    tasks = [
        task("build", steps = ["go build -o $DESTDIR/usr/bin/myapp ./cmd/myapp"]),
    ],
    services = ["myapp"],
    conffiles = ["/etc/myapp/config.toml"],
)
