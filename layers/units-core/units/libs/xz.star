load("//classes/autotools.star", "autotools")

autotools(
    name = "xz",
    version = "5.6.3",
    source = "https://github.com/tukaani-project/xz.git",
    tag = "v5.6.3",
    license = "LGPL-2.1-or-later",
    description = "XZ Utils compression library and tools",
    configure_args = ["--disable-nls"],
    build = [
        # xz's configure.ac uses AM_GNU_GETTEXT which requires gettext m4
        # macros. Stub them out so autoreconf succeeds without gettext installed.
        "mkdir -p m4 && printf '%s\\n' 'AC_DEFUN([AM_GNU_GETTEXT_REQUIRE_VERSION],[])' 'AC_DEFUN([AM_GNU_GETTEXT_VERSION],[])' 'AC_DEFUN([AM_GNU_GETTEXT],[])' > m4/gettext-stub.m4",
        "test -f configure || AUTOPOINT=true autoreconf -fi",
        "./configure --prefix=$PREFIX --disable-nls",
        "make -j$NPROC ACLOCAL=true AUTOCONF=true AUTOMAKE=true AUTOHEADER=true MAKEINFO=true",
        "make DESTDIR=$DESTDIR install ACLOCAL=true AUTOCONF=true AUTOMAKE=true AUTOHEADER=true MAKEINFO=true",
    ],
)
