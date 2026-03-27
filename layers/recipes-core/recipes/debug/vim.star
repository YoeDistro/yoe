package(
    name = "vim",
    version = "9.1.0",
    source = "https://github.com/vim/vim.git",
    tag = "v9.1.0",
    license = "Vim",
    description = "Vi IMproved text editor",
    deps = ["ncurses"],
    runtime_deps = ["ncurses"],
    build = [
        "./configure --prefix=$PREFIX --with-features=normal --disable-gui --without-x",
        "make -j$NPROC",
        "make DESTDIR=$DESTDIR install",
    ],
)
