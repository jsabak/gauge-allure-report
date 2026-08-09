# SPDX-License-Identifier: Apache-2.0
[CmdletBinding()]
param(
    [ValidateSet("python", "js")][string]$Runner = "python",
    [string]$Version = "",
    [string]$PythonCommand = "",
    [switch]$SkipAllure
)
$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = (Get-Content -LiteralPath (Join-Path $ProjectRoot "plugin.json") -Raw | ConvertFrom-Json).version
}
$RunRoot = Join-Path $ProjectRoot (".tools\e2e\" + [guid]::NewGuid().ToString("N"))
$GaugeHome = Join-Path $RunRoot "gauge-home"
$OriginalGaugeHome = $env:GAUGE_HOME
$OriginalGaugePythonCommand = $env:GAUGE_PYTHON_COMMAND
$DefaultGaugeHome = if ($OriginalGaugeHome) { $OriginalGaugeHome } else { Join-Path $env:APPDATA "gauge" }
New-Item -ItemType Directory -Path $GaugeHome -Force | Out-Null
$DefaultGaugeConfig = Join-Path $DefaultGaugeHome "config"
if (Test-Path -LiteralPath $DefaultGaugeConfig) {
    Copy-Item -LiteralPath $DefaultGaugeConfig -Destination $GaugeHome -Recurse -Force
}

function Install-Runner {
    param([string]$Name)
    $Source = Join-Path $DefaultGaugeHome ("plugins\" + $Name)
    $TargetParent = Join-Path $GaugeHome "plugins"
    if (Test-Path -LiteralPath $Source) {
        New-Item -ItemType Directory -Path $TargetParent -Force | Out-Null
        Copy-Item -LiteralPath $Source -Destination $TargetParent -Recurse -Force
        Write-Output "Copied installed Gauge $Name runner into isolated GAUGE_HOME."
        return
    }
    gauge install $Name
    if ($LASTEXITCODE -ne 0) { throw "Could not install Gauge $Name runner" }
}

function Copy-Fixture {
    param([string]$Source, [string]$Name)
    $Target = Join-Path $RunRoot $Name
    Copy-Item -LiteralPath $Source -Destination $Target -Recurse
    return $Target
}

function Read-ResultSet {
    param([string]$Fixture)
    $Results = Join-Path $Fixture "reports\allure-results"
    if (-not (Test-Path -LiteralPath $Results)) { throw "Allure results were not created: $Results" }
    return @(Get-ChildItem -LiteralPath $Results -Filter "*-result.json" | ForEach-Object { Get-Content -LiteralPath $_.FullName -Raw | ConvertFrom-Json })
}

function Assert-SyntheticFailure {
    param([string]$Fixture, [string]$Pattern, [bool]$RequireNonZero)
    Push-Location $Fixture
    try {
        gauge run specs --simple-console
        if ($RequireNonZero -and $LASTEXITCODE -eq 0) { throw "$Pattern fixture unexpectedly returned success" }
    } finally {
        Pop-Location
    }
    $Values = @(Read-ResultSet $Fixture)
    if (-not ($Values | Where-Object { $_.status -eq "broken" -and $_.name -match $Pattern })) {
        throw "$Pattern fixture did not produce a matching broken Allure result"
    }
}

try {
    & (Join-Path $PSScriptRoot "install-local.ps1") -Version $Version -GaugeHome $GaugeHome
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    $env:GAUGE_HOME = $GaugeHome
    Install-Runner $Runner

    $FixtureSource = Join-Path $ProjectRoot ("test\e2e\gauge-" + $Runner + "\projects\main")
    $Fixture = Copy-Fixture $FixtureSource ("gauge-" + $Runner + "-main")
    if ($Runner -eq "python") {
        if ($PythonCommand) {
            if (-not (Test-Path -LiteralPath $PythonCommand)) { throw "Python command does not exist: $PythonCommand" }
            $PythonPath = (Resolve-Path -LiteralPath $PythonCommand).Path
        } else {
            $Python = Get-Command python -ErrorAction SilentlyContinue
            if (-not $Python) { $Python = Get-Command py -ErrorAction SilentlyContinue }
            if (-not $Python) { throw "Python is required for the primary Gauge Python E2E test" }
            $PythonPath = $Python.Source
        }
        $env:GAUGE_PYTHON_COMMAND = $PythonPath
        & $PythonPath -m pip install --disable-pip-version-check -r (Join-Path $Fixture "requirements.txt")
        if ($LASTEXITCODE -ne 0) { throw "Installing Gauge Python fixture requirements failed" }
    }

    Push-Location $Fixture
    try {
        if ($Runner -eq "python") {
            gauge run specs -p -n 2 --max-retries-count 2 --retry-only retry --simple-console
            $GaugeExit = $LASTEXITCODE
            if ($GaugeExit -eq 0) { throw "The exhaustive Python fixture was expected to contain failures" }
        } else {
            gauge run specs --simple-console
            $GaugeExit = $LASTEXITCODE
            if ($GaugeExit -ne 0) { throw "The JavaScript runner-independence smoke fixture failed" }
        }
    } finally {
        Pop-Location
    }

    $ResultsPath = Join-Path $Fixture "reports\allure-results"
    $Results = @(Read-ResultSet $Fixture)
    $Containers = @(Get-ChildItem -LiteralPath $ResultsPath -Filter "*-container.json" | ForEach-Object { Get-Content -LiteralPath $_.FullName -Raw | ConvertFrom-Json })
    if ($Results.Count -lt 1 -or $Containers.Count -lt 3) { throw "Insufficient native artifacts: results=$($Results.Count) containers=$($Containers.Count)" }
    if ($Runner -eq "python") {
        foreach ($Expected in @("passed", "failed", "broken", "skipped")) {
            if ($Expected -notin @($Results.status)) { throw "Missing $Expected result" }
        }
        if (-not ($Results.parameters | Where-Object { $_.name -eq "retry" })) { throw "Retry attempt parameter was not emitted" }
        $AttachmentSources = @($Results.steps.attachments.source) + @($Results.steps.steps.attachments.source) + @($Containers.befores.attachments.source) + @($Containers.afters.attachments.source)
        $AttachmentSources = @($AttachmentSources | Where-Object { $_ })
        if ($AttachmentSources.Count -lt 1) { throw "No message or screenshot attachment was emitted" }
        if (-not (Test-Path -LiteralPath (Join-Path $ResultsPath "environment.properties"))) { throw "environment.properties is missing" }
        if (-not (Test-Path -LiteralPath (Join-Path $ResultsPath "categories.json"))) { throw "categories.json is missing" }
        & $PythonPath (Join-Path $ProjectRoot "test\e2e\assert_results.py") main-python $ResultsPath
        if ($LASTEXITCODE -ne 0) { throw "Deep native Allure assertions failed" }

        $PythonRoot = Join-Path $ProjectRoot "test\e2e\gauge-python\projects"
        $ParseFixture = Copy-Fixture (Join-Path $PythonRoot "parse-error") "gauge-python-parse-error"
        $MissingFixture = Copy-Fixture (Join-Path $PythonRoot "missing-step") "gauge-python-missing-step"
        $NoMatchFixture = Copy-Fixture (Join-Path $PythonRoot "no-match") "gauge-python-no-match"
        Assert-SyntheticFailure $ParseFixture "Parse error" $true
        Assert-SyntheticFailure $MissingFixture "Validation error" $false
        Push-Location $NoMatchFixture
        try {
            gauge run specs --tags selected-nowhere --simple-console
            if ($LASTEXITCODE -ne 0) { throw "No-match fixture failed at the Gauge infrastructure level" }
        } finally {
            Pop-Location
        }
        $NoMatchResults = Join-Path $NoMatchFixture "reports\allure-results"
        if (Test-Path -LiteralPath $NoMatchResults) {
            $NoMatch = @(Read-ResultSet $NoMatchFixture)
            if (-not ($NoMatch | Where-Object { $_.status -eq "skipped" -and $_.name -match "No matching" })) { throw "No-match fixture did not produce a skipped synthetic result" }
        } else {
            Write-Warning "Gauge Core returned before starting reporters for a zero-match tag filter; no protocol callback was possible."
        }
    } elseif ("passed" -notin @($Results.status)) {
        throw "JavaScript smoke fixture did not produce a passed result"
    }

    if (-not $SkipAllure) {
        & (Join-Path $PSScriptRoot "test-allure.ps1") -Major 2 -Results $ResultsPath
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        & (Join-Path $PSScriptRoot "test-allure.ps1") -Major 3 -Results $ResultsPath
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    Write-Output "Gauge $Runner E2E passed native artifact assertions. Diagnostics: $RunRoot"
} catch {
    Write-Error $_
    Write-Output "Diagnostics retained at $RunRoot"
    exit 1
} finally {
    $env:GAUGE_HOME = $OriginalGaugeHome
    $env:GAUGE_PYTHON_COMMAND = $OriginalGaugePythonCommand
}
