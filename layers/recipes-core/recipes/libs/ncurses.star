load("//classes/autotools.star", "autotools")

autotools(
    name = "ncurses",
    version = "6.5",
    source = "https://github.com/mirror/ncurses.git",
    tag = "v6.5",
    license = "MIT",
    description = "Terminal handling library",
)
