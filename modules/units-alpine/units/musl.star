load("@units-alpine//classes/alpine_pkg.star", "alpine_pkg")

alpine_pkg(
    name = "musl",
    version = "1.2.5-r11",
    license = "MIT",
    description = "musl libc shared library (Alpine v3.21)",
    sha256 = {
        "x86_64": "61e84757a8bfbc0d7fa8f4ce6de9cd4d791714369d78f6a08e5b03510fb2a623",
        "arm64":  "721010e6bff908878d9c527428598661be59dde0d9f013f8431d01fd4dd16652",
    },
)
