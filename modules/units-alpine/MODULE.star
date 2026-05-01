module_info(
    name = "units-alpine",
    description = "Wraps prebuilt Alpine Linux packages as yoe units. The Alpine release pinned in classes/alpine_pkg.star MUST match the alpine: tag in @units-core's toolchain-musl Dockerfile — packages from this module are ABI-coupled to the toolchain libc.",
)
