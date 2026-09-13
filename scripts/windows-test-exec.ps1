param([string]$Executable, [string]$WorkingDirectory)

$ErrorActionPreference = 'Stop'
$testArguments = $args
$runDirectory = Join-Path ([IO.Path]::GetTempPath()) ('nmf-native-tests-' + [Guid]::NewGuid().ToString('N'))
$exitCode = 1
try {
    New-Item -ItemType Directory -Path $runDirectory | Out-Null
    $nativeExecutable = Join-Path $runDirectory 'suite.exe'
    Copy-Item -LiteralPath $Executable -Destination $nativeExecutable
    $env:TEMP = $runDirectory
    $env:TMP = $runDirectory
    Push-Location -LiteralPath $WorkingDirectory
    try {
        # Wait explicitly: PowerShell can return before a GUI-subsystem test
        # executable exits, which would race output and temporary-file cleanup.
        $quoted = @($testArguments | ForEach-Object {
            '"' + (($_ -replace '(\\*)"', '$1$1\"') -replace '(\\+)$', '$1$1') + '"'
        })
        $startOptions = @{
            FilePath = $nativeExecutable
            WorkingDirectory = $WorkingDirectory
            NoNewWindow = $true
            Wait = $true
            PassThru = $true
        }
        if ($quoted.Count -gt 0) { $startOptions.ArgumentList = $quoted -join ' ' }
        $process = Start-Process @startOptions
        $exitCode = $process.ExitCode
        if ($exitCode -ne 0) {
            [Console]::Error.WriteLine("Windows test process exited with code $exitCode")
        }
    } finally {
        Pop-Location
    }
} finally {
    if (Test-Path -LiteralPath $runDirectory) {
        Remove-Item -LiteralPath $runDirectory -Recurse -Force
    }
}
exit $exitCode
