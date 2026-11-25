#Requires -Version 7.0
$ErrorActionPreference = 'Stop'
$root = Join-Path ([IO.Path]::GetTempPath()) ('jindi-sign-fixture-' + [Guid]::NewGuid().ToString('N'))
$null = [IO.Directory]::CreateDirectory($root)
$file = Join-Path $root '示例 应用.exe'
$mockTool = Join-Path $root 'fake-signtool.ps1'
$sign = Join-Path $PSScriptRoot 'sign-windows-release.ps1'
$thumbprint = 'A' * 40
$env:JD_SIGN_THUMBPRINT = $thumbprint
$env:JD_SIGNTOOL = $mockTool
$env:JD_SIGN_MACHINE_STORE = $null
$env:JD_TEST_SIGN_LOG = Join-Path $root 'calls.ndjson'
[IO.File]::WriteAllText($file, 'unsigned')
@'
[IO.File]::AppendAllText($env:JD_TEST_SIGN_LOG, (ConvertTo-Json -InputObject $args -Compress) + [Environment]::NewLine)
if ($args[0] -eq 'verify' -and $env:JD_TEST_VERIFY_STATUS) { exit [int]$env:JD_TEST_VERIFY_STATUS }
if ($args[0] -eq 'sign') { [IO.File]::AppendAllText($args[-1], ' signed') }
exit 0
'@ | Set-Content -LiteralPath $mockTool -Encoding utf8

function Get-AuthenticodeSignature([string]$LiteralPath) {
    [pscustomobject]@{
        Status = 'Valid'
        SignatureType = if ($env:JD_TEST_CATALOG_SIGNER) { 'Catalog' } else { 'Authenticode' }
        SignerCertificate = @{ Thumbprint = if ($env:JD_TEST_WRONG_SIGNER) { 'B' * 40 } else { $thumbprint } }
        TimeStamperCertificate = if (-not $env:JD_TEST_NO_TIMESTAMP) { @{ Thumbprint = 'C' * 40 } }
    }
}

function Assert-SigningFailure([scriptblock]$Operation, [string]$Message) {
    $errorMessage = $null
    try { & $Operation } catch { $errorMessage = $_.Exception.Message }
    if ($errorMessage -notlike "*$Message*") { throw "Expected failure '$Message', received '$errorMessage'" }
}

try {
    & $sign $file
    $calls = @(Get-Content -LiteralPath $env:JD_TEST_SIGN_LOG)
    $signArgs = ConvertFrom-Json -InputObject $calls[0] -NoEnumerate
    $verifyArgs = ConvertFrom-Json -InputObject $calls[1] -NoEnumerate
    if ($signArgs[0] -cne 'sign' -or $signArgs[3] -cne '/sha1' -or
        $signArgs[4] -cne $thumbprint -or $signArgs[-1] -cne $file -or
        '/p' -in $signArgs -or '/f' -in $signArgs -or '/sm' -in $signArgs -or
        ($verifyArgs -join ' ') -cne "verify /pa /all /tw $file") {
        throw 'The generated SignTool arguments differ from the certificate-store signing contract'
    }
    $env:JD_SIGN_MACHINE_STORE = '1'
    & $sign $file
    $calls = @(Get-Content -LiteralPath $env:JD_TEST_SIGN_LOG)
    if ('/sm' -notin (ConvertFrom-Json -InputObject $calls[2] -NoEnumerate)) { throw 'Missing machine store selection' }

    $env:JD_TEST_WRONG_SIGNER = '1'
    Assert-SigningFailure { & $sign $file -VerifyOnly } 'Unexpected signer or missing timestamp'
    $env:JD_TEST_WRONG_SIGNER = $null
    $env:JD_TEST_NO_TIMESTAMP = '1'
    Assert-SigningFailure { & $sign $file -VerifyOnly } 'Unexpected signer or missing timestamp'
    $env:JD_TEST_NO_TIMESTAMP = $null
    $env:JD_TEST_CATALOG_SIGNER = '1'
    Assert-SigningFailure { & $sign $file -VerifyOnly } 'Unexpected signer or missing timestamp'
    $env:JD_TEST_CATALOG_SIGNER = $null
    $env:JD_TEST_VERIFY_STATUS = '2'
    Assert-SigningFailure { & $sign $file -VerifyOnly } 'Signature or timestamp verification failed'
    $env:JD_TEST_VERIFY_STATUS = $null

    if ($IsWindows) {
        $buildRoot = Join-Path $root 'source'
        $original = Join-Path $buildRoot 'build/bin/JindiDrama.exe'
        $null = [IO.Directory]::CreateDirectory((Split-Path $original -Parent))
        $null = [IO.Directory]::CreateDirectory((Join-Path $buildRoot 'build/windows/installer'))
        [IO.File]::WriteAllText($original, 'unsigned')
        function makensis {
            if ($env:JD_TEST_FAIL_NSIS) { $global:LASTEXITCODE = 2; return }
            $cwd = (Get-Location).Path
            $uninstaller = Join-Path $cwd 'uninstall.exe'
            [IO.File]::WriteAllText($uninstaller, 'mock uninstaller')
            if (-not $env:JD_TEST_SKIP_NSIS_HOOK) {
                & (Join-Path $cwd '../../../scripts/sign-windows-release.ps1') $uninstaller -Uninstaller
            }
            [IO.File]::WriteAllText((Join-Path $cwd '../../bin/JindiDrama-amd64-installer.exe'), 'mock installer')
            $global:LASTEXITCODE = 0
        }
        $package = Join-Path $PSScriptRoot 'sign-windows-package.ps1'
        $published = Join-Path $root 'signed'
        $env:JD_TEST_FAIL_NSIS = '1'
        Assert-SigningFailure {
            & $package -SigningThumbprint $thumbprint -SignTool $mockTool -BuildRoot $buildRoot -OutputDirectory $published
        } 'NSIS signed installer rebuild failed'
        if (Test-Path -LiteralPath $published) { throw 'Signing failure exposed partial output' }
        $env:JD_TEST_FAIL_NSIS = $null
        $env:JD_TEST_SKIP_NSIS_HOOK = '1'
        Assert-SigningFailure {
            & $package -SigningThumbprint $thumbprint -SignTool $mockTool -BuildRoot $buildRoot -OutputDirectory $published
        } 'NSIS did not run the verified uninstaller signing hook'
        if (Test-Path -LiteralPath $published) { throw 'A missing uninstall hook exposed partial output' }
        $env:JD_TEST_SKIP_NSIS_HOOK = $null
        & $package -SigningThumbprint $thumbprint -SignTool $mockTool -BuildRoot $buildRoot -OutputDirectory $published
        if ((Get-ChildItem -LiteralPath $published -File).Count -ne 2 -or
            [IO.File]::ReadAllText((Join-Path $published 'JindiDrama.exe')) -cne 'unsigned signed' -or
            [IO.File]::ReadAllText($original) -cne 'unsigned') {
            throw 'Package staging did not protect both outputs'
        }
    }
    Write-Host 'Windows store signing command and verification fixtures passed'
} finally {
    Remove-Item -LiteralPath $root -Recurse -Force
}
