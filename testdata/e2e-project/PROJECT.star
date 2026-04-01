project(
    name = "e2e-test",
    version = "0.1.0",
    defaults = defaults(machine = "qemu-x86_64", image = "base-image"),
    repository = repository(path = "repo"),
    cache = cache(path = "build/cache"),
    modules = [
        module("github.com/YoeDistro/yoe-ng",
              local = "../..",
              path = "modules/units-core"),
        module("github.com/YoeDistro/yoe-ng",
              local = "../..",
              path = "modules/units-rpi"),
    ],
)
