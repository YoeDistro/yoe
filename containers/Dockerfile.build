# Yoe-NG build container (Tier 0)
# Version: 14
#
# Tools-only container for build execution. The yoe CLI runs on the host
# and invokes this container only for build commands that need the
# toolchain (gcc, bwrap, mkfs, etc.). No yoe binary inside.

FROM alpine:3.21

# Common packages (all architectures)
RUN apk add --no-cache \
    bubblewrap \
    apk-tools \
    git \
    tar \
    gzip \
    bzip2 \
    xz \
    patch \
    coreutils \
    bash \
    make \
    gcc \
    g++ \
    musl-dev \
    linux-headers \
    sfdisk \
    e2fsprogs \
    dosfstools \
    mtools \
    losetup \
    mount \
    autoconf \
    automake \
    libtool \
    perl \
    bc \
    flex \
    bison \
    elfutils-dev \
    openssl-dev \
    diffutils \
    findutils \
    sed \
    python3 \
    py3-setuptools \
    cmake

# x86-only packages (bootloader, firmware) — skip on other arches
RUN if [ "$(uname -m)" = "x86_64" ]; then \
        apk add --no-cache syslinux ovmf; \
    fi

# Create /build for bwrap bind mounts
RUN mkdir -p /build/src /build/destdir

# Create loop device nodes (needed for losetup in --privileged mode)
RUN for i in $(seq 0 7); do mknod -m 0660 /dev/loop$i b 7 $i 2>/dev/null || true; done

WORKDIR /project
