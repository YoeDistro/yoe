project(name = "name-shadowing", version = "0.1.0",
    defaults = defaults(machine = "qemu-x86_64"),
    modules = [
        module("https://example.com/base.git", local = "modules/base"),
        module("https://example.com/override.git", local = "modules/override"),
    ],
)
