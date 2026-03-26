def autotools(name, version, source, sha256="", deps=[], runtime_deps=[],
              configure_args=[], patches=[], services=[], conffiles=[],
              license="", description="", **kwargs):
    build = [
        "./configure --prefix=$PREFIX " + " ".join(configure_args),
        "make -j$NPROC",
        "make DESTDIR=$DESTDIR install",
    ]
    package(
        name=name, version=version, source=source, sha256=sha256,
        deps=deps, runtime_deps=runtime_deps, patches=patches,
        build=build, services=services, conffiles=conffiles,
        license=license, description=description, **kwargs,
    )
