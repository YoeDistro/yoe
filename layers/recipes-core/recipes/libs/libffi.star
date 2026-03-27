load("//classes/autotools.star", "autotools")

autotools(
    name = "libffi",
    version = "3.4.6",
    source = "https://github.com/libffi/libffi.git",
    tag = "v3.4.6",
    license = "MIT",
    description = "Foreign function interface library",
)
