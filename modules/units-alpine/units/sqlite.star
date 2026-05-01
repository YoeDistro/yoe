load("@units-alpine//classes/alpine_pkg.star", "alpine_pkg")

# Alpine's `sqlite` package ships the sqlite3 CLI with libsqlite3 linked
# statically, so it does NOT depend on sqlite-libs — its runtime deps are
# just musl + readline. Listing sqlite-libs here would still be harmless
# (apk dedupes), but we mirror the upstream APKINDEX D: line for clarity.
alpine_pkg(
    name = "sqlite",
    version = "3.48.0-r4",
    license = "blessing",
    description = "SQLite command-line shell (Alpine v3.21)",
    runtime_deps = ["musl", "readline"],
    sha256 = {
        "x86_64": "fe7bbf2257d73e26bf248c5a11752a328caf3d71c1cb3003f5e1fc1406266b27",
        "arm64":  "8f818aedd63234cef8cd564947115f24b51fd313540a5b6c670b727608251a70",
    },
)
