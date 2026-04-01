load("//classes/image.star", "image")

image(
    name = "base-image",
    artifacts = ["base-files", "busybox", "linux"],
    hostname = "yoe",
)
