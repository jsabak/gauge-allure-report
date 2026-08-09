#!/usr/bin/env sh
# SPDX-License-Identifier: Apache-2.0
set -eu
version="${VERSION:-0.1.0}"
commit="${COMMIT:-unknown}"
build_date="${BUILD_DATE:-unknown}"
dirty="${DIRTY:-unknown}"
mkdir -p bin
go build -trimpath -ldflags "-s -w -X github.com/jsabak/gauge-allure-report/internal/version.Version=$version -X github.com/jsabak/gauge-allure-report/internal/version.Commit=$commit -X github.com/jsabak/gauge-allure-report/internal/version.BuildDate=$build_date -X github.com/jsabak/gauge-allure-report/internal/version.Dirty=$dirty -X github.com/jsabak/gauge-allure-report/internal/version.PackageMarker=gauge-allure-report-package-version:$version" -o bin/allure-report ./cmd/allure-report
bin/allure-report --version
