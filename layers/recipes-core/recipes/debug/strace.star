load("//classes/autotools.star", "autotools")

autotools(
    name = "strace",
    version = "6.12",
    source = "https://github.com/strace/strace.git",
    tag = "v6.12",
    license = "LGPL-2.1-or-later",
    description = "System call tracer for Linux",
)
