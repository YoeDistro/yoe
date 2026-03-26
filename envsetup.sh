#!/usr/bin/env bash
# this file should be sourced (.), not run as a script

OE_BASE=$(readlink -f $(dirname ${BASH_SOURCE[0]:-$0}))

yoe_build() {
  CGO_ENABLED=0 go build -o "${OE_BASE}/yoe" "${OE_BASE}/cmd/yoe" || return 1
}

yoe_test() {
  (cd "${OE_BASE}" && go test ./...) || return 1
}

yoe_format() {
  (cd "${OE_BASE}" && prettier --write "**/*.md") || return 1
}

yoe_format_check() {
  (cd "${OE_BASE}" && prettier --check "**/*.md") || return 1
}

yoe_sloc() {
  (cd "${OE_BASE}" && scc --count-as 'star:py') || return 1
}
