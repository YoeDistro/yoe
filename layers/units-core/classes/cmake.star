def cmake(name, version, source, sha256="", deps=[], runtime_deps=[],
          cmake_args=[], patches=[], services=[], conffiles=[],
          license="", description="", **kwargs):
    build = [
        "cmake -B build -S . -DCMAKE_INSTALL_PREFIX=$PREFIX " +
            " ".join(["-D" + a for a in cmake_args]),
        "cmake --build build -j$NPROC",
        "DESTDIR=$DESTDIR cmake --install build",
    ]
    unit(
        name=name, version=version, source=source, sha256=sha256,
        deps=deps, runtime_deps=runtime_deps, patches=patches,
        build=build, services=services, conffiles=conffiles,
        license=license, description=description, **kwargs,
    )
