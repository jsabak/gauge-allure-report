#!/usr/bin/env sh
# SPDX-License-Identifier: Apache-2.0
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
actual=$(mktemp)
expected=$(mktemp)
trap 'rm -f "$actual" "$expected"' EXIT HUP INT TERM
cd "$root"
go list -deps -f '{{with .Module}}{{.Path}},{{.Version}}{{end}}' ./... | awk 'NF && $0 !~ /^github.com\/jsabak\/gauge-allure-report,$/' | sort -u > "$actual"
tail -n +2 licenses/go-modules.csv | cut -d, -f1,2 | sort -u > "$expected"
diff -u "$expected" "$actual"
awk -F, 'NR > 1 && $3 !~ /^(Apache-2.0|BSD-3-Clause|MIT OR Apache-2.0)$/ { print "unapproved license: " $0 > "/dev/stderr"; bad=1 } END { exit bad }' licenses/go-modules.csv
echo "Dependency module set and licenses verified."
