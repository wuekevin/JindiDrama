#Requires -Version 7.0
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'smoke-windows-session.ps1')

$root = Join-Path ([IO.Path]::GetTempPath()) ('jindi-session-fixture-' + [Guid]::NewGuid().ToString('N'))
[void][IO.Directory]::CreateDirectory($root)
$marker = Join-Path $root 'run-fixture.session'
$version = 'v1.2.3'

function Write-Fixture($Stage, $System, $Architecture, $Version) {
    $record = @{ version = $Version; os = $System; arch = $Architecture } | ConvertTo-Json -Compress
    [IO.File]::WriteAllText($marker, $Stage + $record)
}

function Assert-Rejected([string]$Message, [string]$Expected = $version) {
    $errorMessage = $null
    try { [void](Test-PageReady $root $Expected) } catch { $errorMessage = $_.Exception.Message }
    if ($errorMessage -notlike "*$Message*") { throw "Expected failure '$Message', received '$errorMessage'" }
}

try {
    if (Test-PageReady $root $version) { throw 'Missing marker was treated as ready' }
    Write-Fixture 'S' 'windows' 'amd64' $version
    if (Test-PageReady $root $version) { throw 'Starting marker was treated as ready' }
    Write-Fixture 'R' 'windows' 'amd64' $version
    if (-not (Test-PageReady $root $version)) { throw 'Valid marker was rejected' }

    Write-Fixture 'R' 'darwin' 'amd64' $version
    Assert-Rejected 'expected windows/amd64'
    Write-Fixture 'R' 'windows' 'arm64' $version
    Assert-Rejected 'expected windows/amd64'
    Write-Fixture 'R' 'windows' 'amd64' 'v1.2.2'
    Assert-Rejected 'expected v1.2.3'
    if (-not (Test-PageReady $root)) { throw 'A manual run without an expected version failed' }

    $long = 'v1.2.3+' + ('x' * 121)
    Write-Fixture 'R' 'windows' 'amd64' $long
    if (-not (Test-PageReady $root $long)) { throw 'Maximum complete version failed' }
    $long += 'x'
    Write-Fixture 'R' 'windows' 'amd64' ($long.Substring(0, 125) + '...')
    Assert-Rejected 'marker limit' $long
    foreach ($invalid in @(
        'R{"version":"v1.2.3","os":[],"arch":"amd64"}',
        'R{"version":"v1.2.3","os":"windows","arch":["amd64"]}',
        'R[{"version":"v1.2.3","os":"windows","arch":"amd64"}]'
    )) {
        [IO.File]::WriteAllText($marker, $invalid)
        Assert-Rejected 'missing application identity'
    }
    [IO.File]::WriteAllText($marker, ('R' + ('x' * 512)))
    Assert-Rejected 'invalid size'
    Write-Host 'Windows runtime identity marker cases verified'
} finally {
    Remove-Item -LiteralPath $root -Recurse -Force
}
