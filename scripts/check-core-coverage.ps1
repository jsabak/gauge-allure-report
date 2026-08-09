# SPDX-License-Identifier: Apache-2.0
[CmdletBinding()]
param([double]$Threshold = 85)
$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$Go = Get-Command go -ErrorAction SilentlyContinue
if (-not $Go) { $GoPath = Join-Path $ProjectRoot ".tools\go\bin\go.exe" } else { $GoPath = $Go.Source }
if (-not (Test-Path -LiteralPath $GoPath)) { throw "Go was not found" }
$env:GOCACHE = Join-Path $ProjectRoot ".tools\cache\build"
$env:GOMODCACHE = Join-Path $ProjectRoot ".tools\cache\modules"
Push-Location $ProjectRoot
try {
    foreach ($Package in @("mapping", "identity", "config", "collector", "output")) {
        $Lines = @(& $GoPath test -count=1 -cover ("./internal/" + $Package) 2>&1)
        $Lines | Write-Output
        if ($LASTEXITCODE -ne 0) { throw "Tests failed for internal/$Package" }
        $Match = [regex]::Match(($Lines -join "`n"), "coverage:\s+([0-9.]+)% of statements")
        if (-not $Match.Success) { throw "Could not read coverage for internal/$Package" }
        $Actual = [double]::Parse($Match.Groups[1].Value, [Globalization.CultureInfo]::InvariantCulture)
        if ($Actual -lt $Threshold) { throw "internal/$Package coverage $Actual% is below $Threshold%" }
    }
} finally {
    Pop-Location
}
