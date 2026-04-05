def container(name, version, dockerfile="Dockerfile", scope="noarch", **kwargs):
    unit(
        name=name,
        version=version,
        unit_class="container",
        scope=scope,
        tasks=[
            task("build", fn=lambda: run(
                "docker build -t yoe-ng/%s:%s -f %s/%s %s" % (
                    name, version, name, dockerfile, name),
                host=True,
            )),
        ],
        **kwargs,
    )
