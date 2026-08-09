# SPDX-License-Identifier: Apache-2.0
[CmdletBinding()]
param([ValidateSet(2, 3)][int]$Major, [string]$Results = "reports\allure-results")
$ErrorActionPreference = "Stop"
$env:ALLURE_NO_ANALYTICS = "true"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$ResultsPath = if ([System.IO.Path]::IsPathRooted($Results)) { $Results } else { Join-Path $ProjectRoot $Results }
if (-not (Test-Path -LiteralPath $ResultsPath)) { throw "Allure results do not exist: $ResultsPath" }
$ReportPath = Join-Path $ProjectRoot (".tools\allure-report-" + $Major)
$ToolsRoot = [System.IO.Path]::GetFullPath((Join-Path $ProjectRoot ".tools"))
$ResolvedReport = [System.IO.Path]::GetFullPath($ReportPath)
if (-not $ResolvedReport.StartsWith($ToolsRoot + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing unsafe Allure report cleanup: $ResolvedReport"
}
if (Test-Path -LiteralPath $ResolvedReport) { Remove-Item -LiteralPath $ResolvedReport -Recurse -Force }
if ($Major -eq 2) {
    $Version = "2.45.0"
    $ExpectedHash = "a0a840979b6d212e9eee031563d669985eb353cfde60557343b2903bf08570a6"
    $Downloads = Join-Path $ProjectRoot ".tools\downloads"
    $Archive = Join-Path $Downloads "allure-2.45.0.zip"
    $InstallRoot = Join-Path $ProjectRoot ".tools\allure2\2.45.0"
    New-Item -ItemType Directory -Path $Downloads -Force | Out-Null
    if (-not (Test-Path -LiteralPath $Archive)) {
        Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/allure-framework/allure2/releases/download/$Version/allure-$Version.zip" -OutFile $Archive
    }
    $ActualHash = (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($ActualHash -ne $ExpectedHash) { throw "Allure 2 archive digest mismatch: $ActualHash" }
    $CommandName = if ($IsLinux -or $IsMacOS) { "allure" } else { "allure.bat" }
    $Command = Get-ChildItem -LiteralPath $InstallRoot -Recurse -File -Filter $CommandName -ErrorAction SilentlyContinue | Where-Object { $_.Directory.Name -eq "bin" } | Select-Object -First 1
    if (-not $Command) {
        New-Item -ItemType Directory -Path $InstallRoot -Force | Out-Null
        Expand-Archive -LiteralPath $Archive -DestinationPath $InstallRoot -Force
        $Command = Get-ChildItem -LiteralPath $InstallRoot -Recurse -File -Filter $CommandName | Where-Object { $_.Directory.Name -eq "bin" } | Select-Object -First 1
    }
    if (-not $Command) { throw "Allure 2 executable was not found in the verified archive" }
    if ($IsLinux -or $IsMacOS) { chmod +x $Command.FullName }
    & $Command.FullName generate $ResultsPath --clean -o $ReportPath
} else {
    $ToolingRoot = Join-Path $ProjectRoot "tools\allure"
    & npm.cmd ci --prefix $ToolingRoot --ignore-scripts
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    & npm.cmd exec --prefix $ToolingRoot -- allure generate $ResultsPath -o $ReportPath
}
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
if (-not (Test-Path -LiteralPath (Join-Path $ReportPath "index.html"))) { throw "Allure $Major did not create index.html" }
Write-Output "Allure $Major report generated at $ReportPath"
