unit(
    name = "testlib",
    version = "1.0.0",
    source = "https://example.com/testlib-1.0.tar.gz",
    tasks = [
        task("build", steps = ["make"]),
    ],
)
