project(
    name = "e2e-test",
    version = "0.1.0",
    defaults = defaults(machine = "qemu-x86_64", image = "base-image"),
    repository = repository(path = "build/repo"),
    cache = cache(path = "build/cache"),
    layers = [
        layer("github.com/yoe/recipes-core",
              local = "../../layers/recipes-core"),
    ],
)
