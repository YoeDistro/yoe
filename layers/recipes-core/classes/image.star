def yoe_image(name, version, description="", packages=[], hostname="yoe",
              timezone="UTC", locale="en_US.UTF-8", services=[],
              partitions=[], **kwargs):
    image(
        name=name, version=version, description=description,
        packages=packages, hostname=hostname, timezone=timezone,
        locale=locale, services=services, partitions=partitions,
        **kwargs,
    )
