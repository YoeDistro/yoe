project(
    name = "e2e-test",
    version = "0.1.0",
    defaults = defaults(machine = "qemu-x86_64", image = "base-image"),
    cache = cache(path = "build/cache"),
    tasks_append = [apk_tasks],
    modules = [
        module("github.com/YoeDistro/yoe-ng",
              local = "../..",
              path = "modules/units-core"),
        module("github.com/YoeDistro/yoe-ng",
              local = "../..",
              path = "modules/units-rpi"),
    ],
)
