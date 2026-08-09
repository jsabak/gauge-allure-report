#!/usr/bin/env sh
# SPDX-License-Identifier: Apache-2.0
set -eu

runner="${1:-python}"
version="${VERSION:-0.1.0}"
project_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
run_root="$project_root/.tools/e2e/$(date +%Y%m%d%H%M%S)-$$"
gauge_home="$run_root/gauge-home"
mkdir -p "$gauge_home" "$project_root/bin" "$project_root/dist"
export GAUGE_HOME="$gauge_home"

cd "$project_root"
ldflags="-s -w -X github.com/jsabak/gauge-allure-report/internal/version.Version=$version -X github.com/jsabak/gauge-allure-report/internal/version.Commit=${COMMIT:-unknown} -X github.com/jsabak/gauge-allure-report/internal/version.BuildDate=${BUILD_DATE:-unknown} -X github.com/jsabak/gauge-allure-report/internal/version.Dirty=false -X github.com/jsabak/gauge-allure-report/internal/version.PackageMarker=gauge-allure-report-package-version:$version"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o bin/allure-report ./cmd/allure-report
go run ./build/package -binary bin/allure-report -os linux -arch amd64 -version "$version" -out dist
archive="$project_root/dist/allure-report-$version-linux.x86_64.zip"
go run ./build/verify -archive "$archive" -version "$version"
gauge install allure-report --file "$archive"
gauge install "$runner"

fixture="$run_root/gauge-$runner-main"
cp -R "$project_root/test/e2e/gauge-$runner/projects/main" "$fixture"
if [ "$runner" = python ]; then
  python -m pip install --disable-pip-version-check -r "$fixture/requirements.txt"
fi

set +e
(cd "$fixture" && gauge run specs -p -n 2 --max-retries-count 2 --retry-only retry --simple-console)
main_exit=$?
set -e
if [ "$runner" = python ] && [ "$main_exit" -eq 0 ]; then
  echo "exhaustive Python fixture unexpectedly passed" >&2
  exit 1
fi
if [ "$runner" = js ] && [ "$main_exit" -ne 0 ]; then
  echo "JavaScript smoke fixture failed" >&2
  exit 1
fi
python "$project_root/test/e2e/assert_results.py" "main-$runner" "$fixture/reports/allure-results"

if [ "$runner" = python ]; then
  for name in parse-error missing-step no-match; do
    target="$run_root/gauge-python-$name"
    cp -R "$project_root/test/e2e/gauge-python/projects/$name" "$target"
    set +e
    if [ "$name" = no-match ]; then
      (cd "$target" && gauge run specs --tags selected-nowhere --simple-console)
    else
      (cd "$target" && gauge run specs --simple-console)
    fi
    code=$?
    set -e
    if [ "$name" = parse-error ] && [ "$code" -eq 0 ]; then
      echo "parse-error fixture unexpectedly returned success" >&2
      exit 1
    fi
    case "$name" in
      parse-error)
        python "$project_root/test/e2e/assert_results.py" parse "$target/reports/allure-results"
        ;;
      missing-step)
        python "$project_root/test/e2e/assert_results.py" missing "$target/reports/allure-results"
        ;;
      no-match)
        if [ -d "$target/reports/allure-results" ]; then
          python "$project_root/test/e2e/assert_results.py" no-match "$target/reports/allure-results"
        else
          echo "warning: Gauge Core returned before starting reporters for a zero-match tag filter" >&2
        fi
        ;;
    esac
  done
fi

if [ "${SKIP_ALLURE:-false}" != true ]; then
  export ALLURE_NO_ANALYTICS=true
  allure2_version=2.45.0
  allure2_sha256=a0a840979b6d212e9eee031563d669985eb353cfde60557343b2903bf08570a6
  allure2_archive="$project_root/.tools/downloads/allure-$allure2_version.zip"
  allure2_home="$project_root/.tools/allure2/$allure2_version"
  mkdir -p "$(dirname "$allure2_archive")" "$allure2_home"
  if [ ! -f "$allure2_archive" ]; then
    curl --fail --location --retry 3 "https://github.com/allure-framework/allure2/releases/download/$allure2_version/allure-$allure2_version.zip" --output "$allure2_archive"
  fi
  printf '%s  %s\n' "$allure2_sha256" "$allure2_archive" | sha256sum --check --status
  allure2_command=$(find "$allure2_home" -type f -path '*/bin/allure' -print -quit)
  if [ -z "$allure2_command" ]; then
    unzip -q -o "$allure2_archive" -d "$allure2_home"
    allure2_command=$(find "$allure2_home" -type f -path '*/bin/allure' -print -quit)
  fi
  test -n "$allure2_command"
  chmod +x "$allure2_command"
  "$allure2_command" generate "$fixture/reports/allure-results" --clean -o "$run_root/allure2"
  test -f "$run_root/allure2/index.html"
  npm ci --prefix "$project_root/tools/allure" --ignore-scripts
  npm exec --prefix "$project_root/tools/allure" -- allure generate "$fixture/reports/allure-results" -o "$run_root/allure3"
  test -f "$run_root/allure3/index.html"
fi
echo "Gauge $runner E2E passed; diagnostics: $run_root"
