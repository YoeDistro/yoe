project(
    name = "e2e-test",
    version = "0.1.0",
    defaults = defaults(machine = "qemu-x86_64", image = "base-image"),
    repository = repository(path = "repo"),
    cache = cache(path = "build/cache"),
    layers = [
        layer("github.com/YoeDistro/yoe-ng",
              local = "../..",
              path = "layers/units-core"),
        layer("github.com/YoeDistro/yoe-ng",
              local = "../..",
              path = "layers/units-rpi"),
    ],
)
