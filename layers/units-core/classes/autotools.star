def autotools(name, version, source, sha256="", deps=[], runtime_deps=[],
              configure_args=[], patches=[], services=[], conffiles=[],
              license="", description="", build=[], **kwargs):
    if not build:
        build = [
            # Run autoreconf if configure doesn't exist (git sources).
            # AUTOPOINT=true skips autopoint (gettext) which is not in the container.
            "test -f configure || AUTOPOINT=true autoreconf -fi",
            "./configure --prefix=$PREFIX " + " ".join(configure_args),
            # Override maintainer-mode tools so make doesn't try to re-run
            # versioned autotools (e.g. aclocal-1.16) that aren't in the container
            "make -j$NPROC ACLOCAL=true AUTOCONF=true AUTOMAKE=true AUTOHEADER=true MAKEINFO=true",
            "make DESTDIR=$DESTDIR install ACLOCAL=true AUTOCONF=true AUTOMAKE=true AUTOHEADER=true MAKEINFO=true",
        ]
    unit(
        name=name, version=version, source=source, sha256=sha256,
        deps=deps, runtime_deps=runtime_deps, patches=patches,
        build=build, services=services, conffiles=conffiles,
        license=license, description=description, **kwargs,
    )
