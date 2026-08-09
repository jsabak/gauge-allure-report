# SPDX-License-Identifier: Apache-2.0
[CmdletBinding()]
param()
$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$Go = Get-Command go -ErrorAction SilentlyContinue
if (-not $Go) { $GoPath = Join-Path $ProjectRoot ".tools\go\bin\go.exe" } else { $GoPath = $Go.Source }
$env:GOCACHE = Join-Path $ProjectRoot ".tools\cache\build"
$env:GOMODCACHE = Join-Path $ProjectRoot ".tools\cache\modules"
Push-Location $ProjectRoot
try {
    $Actual = @(& $GoPath list -deps -f '{{with .Module}}{{.Path}},{{.Version}}{{end}}' ./... | Where-Object { $_ -and $_ -ne 'github.com/jsabak/gauge-allure-report,' } | Sort-Object -Unique)
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect compiled modules" }
    $Rows = Import-Csv licenses\go-modules.csv
    $Expected = @($Rows | ForEach-Object { $_.module + ',' + $_.version } | Sort-Object -Unique)
    if (Compare-Object $Expected $Actual) { throw "Compiled dependency modules differ from licenses/go-modules.csv" }
    $Allowed = @('Apache-2.0', 'BSD-3-Clause', 'MIT OR Apache-2.0')
    if ($Rows | Where-Object { $_.license -notin $Allowed }) { throw "Dependency allowlist contains an unapproved license" }
    Write-Output "Dependency module set and licenses verified."
} finally {
    Pop-Location
}
