load("//classes/go.star", "go_binary")

go_binary(
    name = "simpleiot",
    version = "0.18.5",
    source = "https://github.com/simpleiot/simpleiot.git",
    tag = "v0.18.5",
    go_package = "./cmd/siot",
    license = "Apache-2.0",
    description = "IoT application for sensor data, telemetry, configuration, and device management",
)
