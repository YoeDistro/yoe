def autotools(name, version, source, sha256="", deps=[], runtime_deps=[],
              configure_args=[], patches=[], services=[], conffiles=[],
              license="", description="", tasks=[], scope="", **kwargs):
    if not tasks:
        tasks = [
            task("build", steps=[
                "test -f configure || autoreconf -fi",
                "./configure --prefix=$PREFIX " + " ".join(configure_args),
                "make -j$NPROC ACLOCAL=true AUTOCONF=true AUTOMAKE=true AUTOHEADER=true MAKEINFO=true",
                "make DESTDIR=$DESTDIR install ACLOCAL=true AUTOCONF=true AUTOMAKE=true AUTOHEADER=true MAKEINFO=true",
            ]),
        ]
    unit(
        name=name, version=version, source=source, sha256=sha256,
        deps=deps, runtime_deps=runtime_deps, patches=patches,
        tasks=tasks, services=services, conffiles=conffiles,
        license=license, description=description, scope=scope,
        **kwargs,
    )
