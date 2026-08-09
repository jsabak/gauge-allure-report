#!/usr/bin/env sh
# SPDX-License-Identifier: Apache-2.0
set -eu

threshold="${CORE_COVERAGE_THRESHOLD:-85}"
for package in mapping identity config collector output; do
  output=$(go test -count=1 -cover "./internal/$package")
  printf '%s\n' "$output"
  percent=$(printf '%s\n' "$output" | sed -n 's/.*coverage: \([0-9.][0-9.]*\)% of statements.*/\1/p' | tail -n 1)
  if [ -z "$percent" ]; then
    echo "Could not read coverage for internal/$package" >&2
    exit 1
  fi
  awk -v package="$package" -v actual="$percent" -v required="$threshold" 'BEGIN { if (actual + 0 < required + 0) { printf "internal/%s coverage %.1f%% is below %.1f%%\n", package, actual, required > "/dev/stderr"; exit 1 } }'
done
