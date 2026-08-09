# SPDX-License-Identifier: Apache-2.0
[CmdletBinding()]
param([string]$Version = "0.1.0", [string]$GaugeHome = "")
$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
& (Join-Path $PSScriptRoot "build.ps1") -Version $Version
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$Go = Get-Command go -ErrorAction SilentlyContinue
if (-not $Go) { $GoPath = Join-Path $ProjectRoot ".tools\go\bin\go.exe" } else { $GoPath = $Go.Source }
$env:GOCACHE = Join-Path $ProjectRoot ".tools\cache\build"
$env:GOMODCACHE = Join-Path $ProjectRoot ".tools\cache\modules"
Push-Location $ProjectRoot
try {
    & $GoPath run ./build/package -binary bin/allure-report.exe -os windows -arch amd64 -version $Version -out dist
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    & $GoPath run ./build/verify -archive "dist/allure-report-$Version-windows.x86_64.zip" -version $Version
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Pop-Location
}
$Archive = Join-Path $ProjectRoot "dist\allure-report-$Version-windows.x86_64.zip"
if ($GaugeHome) { $env:GAUGE_HOME = $GaugeHome }
gauge install allure-report --file $Archive
