#!/usr/bin/env bash
# this file should be sourced (.), not run as a script

OE_BASE=$(readlink -f $(dirname ${BASH_SOURCE[0]:-$0}))

yoe_format() {
  prettier --write "**/*.md" || return 1
}

yoe_format_check() {
  prettier --check "**/*.md" || return 1
}
