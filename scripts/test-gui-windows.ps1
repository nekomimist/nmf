# Native GUI smoke test in an MXC IsolationSession, never on the host desktop.
[CmdletBinding()]
param(
    [string]$Zig = 'zig',
    [string]$CleanupRun
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'This GUI test requires Windows. Ordinary Linux and cross builds use the Makefile.'
}
$repoRoot = Split-Path -Parent $PSScriptRoot
$sourceDir = Join-Path $PSScriptRoot 'gui-windows'
$cacheDir = Join-Path $repoRoot '.cache/gui-windows'
$sdkDir = Join-Path $cacheDir 'sdk'
$runner = Join-Path $sourceDir 'runner.mjs'
$node = (Get-Command node -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
$npm = (Get-Command npm.cmd -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source

New-Item -ItemType Directory -Path $sdkDir -Force | Out-Null
$lockHash = (Get-FileHash -LiteralPath (Join-Path $sourceDir 'package-lock.json') -Algorithm SHA256).Hash
$marker = Join-Path $sdkDir 'installed-lock.sha256'
if (!(Test-Path -LiteralPath $marker) -or (Get-Content -LiteralPath $marker -Raw).Trim() -ne $lockHash -or
    !(Test-Path -LiteralPath (Join-Path $sdkDir 'node_modules/@microsoft/mxc-sdk/package.json'))) {
    Copy-Item -LiteralPath (Join-Path $sourceDir 'package.json') -Destination $sdkDir
    Copy-Item -LiteralPath (Join-Path $sourceDir 'package-lock.json') -Destination $sdkDir
    & $npm ci --prefix $sdkDir --ignore-scripts --no-audit --no-fund
    if ($LASTEXITCODE -ne 0) { throw 'Could not install the pinned MXC SDK.' }
    Set-Content -LiteralPath $marker -Value $lockHash -Encoding ASCII
}

if ($CleanupRun) {
    & $node $runner cleanup ([IO.Path]::GetFullPath($CleanupRun)) $sdkDir
    exit $LASTEXITCODE
}
& $node $runner check $cacheDir $sdkDir
if ($LASTEXITCODE -ne 0) { throw 'IsolationSession preflight failed.' }
$compiler = Join-Path $env:WINDIR 'Microsoft.NET/Framework64/v4.0.30319/csc.exe'
if (!(Test-Path -LiteralPath $compiler)) { throw 'The Windows .NET Framework C# compiler is required.' }
$runsDir = Join-Path $cacheDir 'runs'
New-Item -ItemType Directory -Path $runsDir -Force | Out-Null
$runDir = Join-Path $runsDir ((Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 8))
Write-Host "GUI test artifacts: $runDir"
& $node $runner prepare $runDir
if ($LASTEXITCODE -ne 0) { throw 'Could not prepare the GUI build overlay.' }
& $compiler /nologo /target:exe /platform:x64 /r:System.Drawing.dll /r:System.Web.Extensions.dll ("/out:" + (Join-Path $runDir 'GuiSmoke.exe')) (Join-Path $sourceDir 'GuiSmoke.cs')
if ($LASTEXITCODE -ne 0) { throw 'Could not compile the isolated input driver.' }
& (Join-Path $PSScriptRoot 'dev-windows.ps1') build -Zig $Zig -Arguments @('-overlay', (Join-Path $runDir 'overlay.json'), '-o', (Join-Path $runDir 'nmf-gui.exe'))
if ($LASTEXITCODE -ne 0) { throw 'Could not build the GUI test executable.' }
$hostSessionId = (Get-Process -Id $PID).SessionId
& $node $runner run $runDir $sdkDir $hostSessionId
exit $LASTEXITCODE
