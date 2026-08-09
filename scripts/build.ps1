# SPDX-License-Identifier: Apache-2.0
[CmdletBinding()]
param(
    [string]$Version = "0.1.0",
    [string]$Commit = "unknown",
    [string]$BuildDate = "unknown",
    [string]$Dirty = "unknown"
)
$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$Go = Get-Command go -ErrorAction SilentlyContinue
if (-not $Go) { $GoPath = Join-Path $ProjectRoot ".tools\go\bin\go.exe" } else { $GoPath = $Go.Source }
if (-not (Test-Path -LiteralPath $GoPath)) { throw "Go was not found; install Go 1.26 or place the portable toolchain in .tools/go" }
$Output = Join-Path $ProjectRoot "bin\allure-report.exe"
New-Item -ItemType Directory -Path (Split-Path -Parent $Output) -Force | Out-Null
$env:GOCACHE = Join-Path $ProjectRoot ".tools\cache\build"
$env:GOMODCACHE = Join-Path $ProjectRoot ".tools\cache\modules"
$Ldflags = "-s -w -X github.com/jsabak/gauge-allure-report/internal/version.Version=$Version -X github.com/jsabak/gauge-allure-report/internal/version.Commit=$Commit -X github.com/jsabak/gauge-allure-report/internal/version.BuildDate=$BuildDate -X github.com/jsabak/gauge-allure-report/internal/version.Dirty=$Dirty -X github.com/jsabak/gauge-allure-report/internal/version.PackageMarker=gauge-allure-report-package-version:$Version"
Push-Location $ProjectRoot
try {
    & $GoPath build -trimpath -ldflags $Ldflags -o $Output ./cmd/allure-report
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    & $Output --version
} finally {
    Pop-Location
}
