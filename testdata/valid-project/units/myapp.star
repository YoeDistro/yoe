go_binary(
    name = "myapp",
    version = "1.2.3",
    description = "Edge data collection service",
    license = "Apache-2.0",
    source = "https://github.com/example/myapp.git",
    tag = "v1.2.3",
    package = "./cmd/myapp",
    services = ["myapp"],
    conffiles = ["/etc/myapp/config.toml"],
)
