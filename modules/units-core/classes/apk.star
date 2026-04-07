def apk_tasks():
    """Return packaging tasks that create an APK and publish it to the repo."""
    return [
        task("package", fn=lambda: _package()),
    ]

def _package():
    result = apk_create()
    apk_publish(result.path)
    sysroot_stage()
