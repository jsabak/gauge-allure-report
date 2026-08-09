# SPDX-License-Identifier: Apache-2.0
[CmdletBinding()]
param(
    [Parameter(Mandatory)][ValidateSet("linux", "darwin", "windows")][string]$OS,
    [Parameter(Mandatory)][ValidateSet("386", "amd64", "arm64")][string]$Arch,
    [string]$Version = "",
    [string]$Commit = "ci",
    [string]$BuildDate = "1970-01-01T00:00:00Z",
    [switch]$Smoke
)
$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = (Get-Content -LiteralPath (Join-Path $ProjectRoot "plugin.json") -Raw | ConvertFrom-Json).version
}
$OriginalGOOS, $OriginalGOARCH, $OriginalCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
$env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $OS, $Arch, "0"
$Suffix = if ($OS -eq "windows") { ".exe" } else { "" }
$Binary = Join-Path $ProjectRoot ("bin\allure-report-" + $OS + "-" + $Arch + $Suffix)
$Ldflags = "-s -w -X github.com/jsabak/gauge-allure-report/internal/version.Version=$Version -X github.com/jsabak/gauge-allure-report/internal/version.Commit=$Commit -X github.com/jsabak/gauge-allure-report/internal/version.BuildDate=$BuildDate -X github.com/jsabak/gauge-allure-report/internal/version.Dirty=false -X github.com/jsabak/gauge-allure-report/internal/version.PackageMarker=gauge-allure-report-package-version:$Version"
Push-Location $ProjectRoot
try {
    New-Item -ItemType Directory -Path (Split-Path -Parent $Binary) -Force | Out-Null
    go build -trimpath -ldflags $Ldflags -o $Binary ./cmd/allure-report
    if ($LASTEXITCODE -ne 0) { throw "Build failed for $OS/$Arch" }
    if ($Smoke) {
        & $Binary --version
        if ($LASTEXITCODE -ne 0) { throw "Native binary smoke failed for $OS/$Arch" }
    }
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $OriginalGOOS, $OriginalGOARCH, $OriginalCGO
    go run ./build/package -binary $Binary -os $OS -arch $Arch -version $Version -out dist
    if ($LASTEXITCODE -ne 0) { throw "Packaging failed for $OS/$Arch" }
    $PackageArch = @{ "386" = "x86"; "amd64" = "x86_64"; "arm64" = "arm64" }[$Arch]
    $Archive = Join-Path $ProjectRoot ("dist\allure-report-$Version-$OS.$PackageArch.zip")
    go run ./build/verify -archive $Archive -version $Version
    if ($LASTEXITCODE -ne 0) { throw "Package verification failed for $OS/$Arch" }
} finally {
    Pop-Location
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $OriginalGOOS, $OriginalGOARCH, $OriginalCGO
}
