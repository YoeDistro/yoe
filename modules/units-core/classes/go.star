load("//classes/apk.star", "apk_tasks")

def go_binary(name, version, source, tag="", sha256="",
              go_package="", deps=[], runtime_deps=[],
              services=[], conffiles=[], environment={},
              license="", description="", tasks=[], scope="",
              container="toolchain-musl", container_arch="host", **kwargs):
    if not go_package:
        go_package = "./cmd/" + name
    if not tasks:
        tasks = [
            task("build", steps=[
                "go build -o $DESTDIR$PREFIX/bin/" + name + " " + go_package,
            ]),
        ]
    tasks = tasks + apk_tasks()
    # Merge class deps with user deps
    all_deps = list(deps)
    if container and container not in all_deps:
        all_deps.append(container)
    unit(
        name=name, version=version, source=source, sha256=sha256,
        tag=tag, deps=all_deps, runtime_deps=runtime_deps,
        tasks=tasks, services=services, conffiles=conffiles,
        license=license, description=description, scope=scope,
        container=container, container_arch=container_arch,
        **kwargs,
    )
