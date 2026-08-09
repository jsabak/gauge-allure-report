# SPDX-License-Identifier: Apache-2.0
[CmdletBinding()]
param(
    [string]$Version = "0.1.0",
    [string]$Commit = "unknown",
    [string]$BuildDate = "unknown",
    [string]$Dirty = "false"
)
$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$Go = Get-Command go -ErrorAction SilentlyContinue
if (-not $Go) { $GoPath = Join-Path $ProjectRoot ".tools\go\bin\go.exe" } else { $GoPath = $Go.Source }
if (-not (Test-Path -LiteralPath $GoPath)) { throw "Go was not found" }
$env:GOCACHE = Join-Path $ProjectRoot ".tools\cache\build"
$env:GOMODCACHE = Join-Path $ProjectRoot ".tools\cache\modules"
$OriginalGOOS, $OriginalGOARCH, $OriginalCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
$Targets = @(
    @{ OS = "linux"; Arch = "386" },
    @{ OS = "linux"; Arch = "amd64" },
    @{ OS = "linux"; Arch = "arm64" },
    @{ OS = "darwin"; Arch = "amd64" },
    @{ OS = "darwin"; Arch = "arm64" },
    @{ OS = "windows"; Arch = "386" },
    @{ OS = "windows"; Arch = "amd64" }
)
$Ldflags = "-s -w -X github.com/jsabak/gauge-allure-report/internal/version.Version=$Version -X github.com/jsabak/gauge-allure-report/internal/version.Commit=$Commit -X github.com/jsabak/gauge-allure-report/internal/version.BuildDate=$BuildDate -X github.com/jsabak/gauge-allure-report/internal/version.Dirty=$Dirty -X github.com/jsabak/gauge-allure-report/internal/version.PackageMarker=gauge-allure-report-package-version:$Version"
Push-Location $ProjectRoot
try {
    foreach ($Target in $Targets) {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $Target.OS, $Target.Arch, "0"
        $Suffix = if ($Target.OS -eq "windows") { ".exe" } else { "" }
        $Binary = Join-Path $ProjectRoot (".tools\package\" + $Target.OS + "-" + $Target.Arch + "\allure-report" + $Suffix)
        New-Item -ItemType Directory -Path (Split-Path -Parent $Binary) -Force | Out-Null
        & $GoPath build -trimpath -ldflags $Ldflags -o $Binary ./cmd/allure-report
        if ($LASTEXITCODE -ne 0) { throw "Build failed for $($Target.OS)/$($Target.Arch)" }
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $OriginalGOOS, $OriginalGOARCH, $OriginalCGO
        & $GoPath run ./build/package -binary $Binary -os $Target.OS -arch $Target.Arch -version $Version -out dist
        if ($LASTEXITCODE -ne 0) { throw "Packaging failed for $($Target.OS)/$($Target.Arch)" }
    }
    Get-ChildItem -LiteralPath (Join-Path $ProjectRoot "dist") -Filter "*.zip" | ForEach-Object {
        & $GoPath run ./build/verify -archive $_.FullName -version $Version
        if ($LASTEXITCODE -ne 0) { throw "Package verification failed: $($_.Name)" }
    }
} finally {
    Pop-Location
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $OriginalGOOS, $OriginalGOARCH, $OriginalCGO
}
