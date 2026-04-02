def cmake(name, version, source, sha256="", deps=[], runtime_deps=[],
          cmake_args=[], patches=[], services=[], conffiles=[],
          license="", description="", tasks=[], scope="", **kwargs):
    if not tasks:
        tasks = [
            task("build", steps=[
                "cmake -B build -S . -DCMAKE_INSTALL_PREFIX=$PREFIX " +
                    " ".join(["-D" + a for a in cmake_args]),
                "cmake --build build -j$NPROC",
                "DESTDIR=$DESTDIR cmake --install build",
            ]),
        ]
    unit(
        name=name, version=version, source=source, sha256=sha256,
        deps=deps, runtime_deps=runtime_deps, patches=patches,
        tasks=tasks, services=services, conffiles=conffiles,
        license=license, description=description, scope=scope,
        **kwargs,
    )
