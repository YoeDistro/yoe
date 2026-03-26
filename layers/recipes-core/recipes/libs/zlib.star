load("//classes/autotools.star", "autotools")

autotools(
    name = "zlib",
    version = "1.3.1",
    source = "https://github.com/madler/zlib.git",
    tag = "v1.3.1",
    license = "Zlib",
    description = "Compression library",
)
