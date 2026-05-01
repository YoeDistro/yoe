load("@units-alpine//classes/alpine_pkg.star", "alpine_pkg")

alpine_pkg(
    name = "sqlite-libs",
    version = "3.48.0-r4",
    license = "blessing",
    description = "SQLite shared library (Alpine v3.21)",
    runtime_deps = ["musl"],
    sha256 = {
        "x86_64": "56b55029cce8d43a67f5076e07c96b68b80b26dc412b205b7f0b5c63704ab234",
        "arm64":  "0e4e80c156f65307d0b35c4caf5771fae2a8c3d7f5ffb642a72d13e9c6d3cd14",
    },
)
