def go_binary(name, version, source, tag="", sha256="",
              go_package="", deps=[], runtime_deps=[],
              services=[], conffiles=[], environment={},
              license="", description="", **kwargs):
    if not go_package:
        go_package = "./cmd/" + name
    build = [
        "go build -o $DESTDIR$PREFIX/bin/" + name + " " + go_package,
    ]
    package(
        name=name, version=version, source=source, sha256=sha256,
        tag=tag, deps=deps, runtime_deps=runtime_deps,
        build=build, services=services, conffiles=conffiles,
        license=license, description=description, **kwargs,
    )
