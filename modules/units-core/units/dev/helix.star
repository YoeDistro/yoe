load("//classes/binary.star", "binary")

# Helix — modal text editor (https://helix-editor.com/). Ships as a
# tar.xz with the `hx` binary plus a `runtime/` tree (themes, queries,
# language grammars). Helix locates its runtime as
# `<canonical-binary-dir>/../lib/helix/runtime`, so install_tree puts
# everything under /usr/lib/helix and the /usr/bin/hx symlink resolves
# back into that directory.
#
# Helix's asset filenames use kernel-style arch tokens, not Go-style.
binary(
    name = "helix",
    version = "25.07.1",
    base_url = "https://github.com/helix-editor/helix/releases/download/{version}",
    asset = "helix-{version}-{arch}-linux.tar.xz",
    arch_map = {
        "x86_64": "x86_64",
        "arm64":  "aarch64",
    },
    sha256 = {
        "x86_64": "3f08e63ecd388fff657ad39722f88bb03dcf326f1f2da2700d99e1dc40ab2e8b",
        "arm64":  "ce23fa8d395e633e3e54c052012f11965d91d8d5c2bfa659685f50430b4f8175",
    },
    install_tree = "$PREFIX/lib/helix",
    binaries = ["hx"],
    license = "MPL-2.0",
    description = "Post-modern modal text editor with built-in LSP support",
)
