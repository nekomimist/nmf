# Native Windows development; release packaging remains in the Linux Makefile.
[CmdletBinding(PositionalBinding = $false)]
param(
    [Parameter(Position = 0)]
    [ValidateSet('build', 'run', 'test', 'vet')]
    [string]$Task = 'build',

    [string]$Zig = 'zig',

    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Arguments = @()
)

$ErrorActionPreference = 'Stop'
# Capture native exit codes explicitly, including in PowerShell 7.3+.
$PSNativeCommandUseErrorActionPreference = $false
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'dev-windows.ps1 requires Windows. Use the Makefile on Linux/WSL.'
}

# Multiple installed versions can match; use the first application on PATH.
$goCommand = (Get-Command go -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
$zigCommand = (Get-Command $Zig -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
$repoRoot = Split-Path -Parent $PSScriptRoot
$savedEnvironment = @{}
$environmentNames = @(
    'GOOS', 'GOARCH', 'CGO_ENABLED', 'CC', 'CXX', 'GOCACHE',
    'ZIG_LOCAL_CACHE_DIR', 'ZIG_GLOBAL_CACHE_DIR'
)
foreach ($name in $environmentNames) {
    $savedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}

$exitCode = 1
Push-Location -LiteralPath $repoRoot
try {
    $hostArch = & $goCommand env GOHOSTARCH
    if ($LASTEXITCODE -ne 0) { throw 'Could not determine the Go host architecture.' }
    $zigArch = switch ($hostArch) {
        'amd64' { 'x86_64' }
        'arm64' { 'aarch64' }
        default { throw "Unsupported Windows host architecture: $hostArch" }
    }
    $env:GOOS = 'windows'
    $env:GOARCH = $hostArch
    $env:CGO_ENABLED = '1'
    $compilerFlags = "-target $zigArch-windows-gnu -Wdeprecated-non-prototype"
    $env:CC = '"' + $zigCommand + '" cc ' + $compilerFlags
    $env:CXX = '"' + $zigCommand + '" c++ ' + $compilerFlags
    # Separate Windows caches from WSL, including inherited Linux cache paths.
    $env:GOCACHE = Join-Path $repoRoot '.cache/go-build-windows'
    $env:ZIG_LOCAL_CACHE_DIR = Join-Path $repoRoot '.cache/zig-local-windows'
    $env:ZIG_GLOBAL_CACHE_DIR = Join-Path $repoRoot '.cache/zig-global-windows'

    $goArguments = @($Task, '-mod=readonly', '-tags', 'migrated_fynedo')
    switch ($Task) {
        'build' {
            New-Item -ItemType Directory -Path (Join-Path $repoRoot 'dist') -Force | Out-Null
            $goArguments += @('-o', (Join-Path $repoRoot 'dist/nmf-dev.exe')) + $Arguments + @('.')
        }
        'run' { $goArguments += @('.') + $Arguments }
        'test' {
            $goArguments += '-count=1'
            if ($Arguments.Count -eq 0) { $Arguments = @('./...') }
            $goArguments += $Arguments
        }
        'vet' {
            if ($Arguments.Count -eq 0) { $Arguments = @('./...') }
            $goArguments += $Arguments
        }
    }
    & $goCommand @goArguments
    $exitCode = $LASTEXITCODE
} finally {
    foreach ($name in $environmentNames) {
        [Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name], 'Process')
    }
    Pop-Location
}
exit $exitCode
