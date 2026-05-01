#!/usr/bin/env bash
# this file should be sourced (.), not run as a script

OE_BASE=$(readlink -f $(dirname ${BASH_SOURCE[0]:-$0}))

yoe_build() {
  local version
  version=$(cd "${OE_BASE}" && git describe --tags --always --dirty 2>/dev/null || echo "dev")
  CGO_ENABLED=0 go build -ldflags "-X main.version=${version}" -o "${OE_BASE}/yoe" "${OE_BASE}/cmd/yoe" || return 1
}

yoe_test() {
  (cd "${OE_BASE}" && go test ./...) || return 1
}

yoe_format() {
  docker run --rm -v "${OE_BASE}:/work" -w /work node:20-alpine \
    npx --yes prettier --write "**/*.md" || return 1
}

yoe_format_check() {
  docker run --rm -v "${OE_BASE}:/work" -w /work node:20-alpine \
    npx --yes prettier --check "**/*.md" || return 1
}

yoe_sloc() {
  (cd "${OE_BASE}" && scc --count-as 'star:py') || return 1
}

yoe_e2e() {
  yoe_build || return 1
  echo "=== e2e: base-image (x86_64) ==="
  (cd "${OE_BASE}/testdata/e2e-project" && "${OE_BASE}/yoe" build --machine qemu-x86_64 base-image) || return 1
  echo "=== e2e: base-image (arm64 cross) ==="
  (cd "${OE_BASE}/testdata/e2e-project" && "${OE_BASE}/yoe" build --machine qemu-arm64 base-image) || return 1
  echo "=== e2e: all passed ==="
}

yoe_e2e_x86_64() {
  yoe_build || return 1
  echo "=== e2e: base-image (x86_64) ==="
  (cd "${OE_BASE}/testdata/e2e-project" && "${OE_BASE}/yoe" build --machine qemu-x86_64 base-image) || return 1
}

yoe_e2e_arm64() {
  yoe_build || return 1
  echo "=== e2e: base-image (arm64 cross) ==="
  (cd "${OE_BASE}/testdata/e2e-project" && "${OE_BASE}/yoe" build --machine qemu-arm64 base-image) || return 1
}
