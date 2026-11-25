param(
    [Parameter(Mandatory)][string]$File,
    [switch]$VerifyOnly,
    [switch]$Uninstaller
)

$ErrorActionPreference = 'Stop'
$thumbprint = $env:JD_SIGN_THUMBPRINT
if (-not $env:JD_SIGNTOOL -or $thumbprint -notmatch '^[0-9A-Fa-f]{40}$') {
    throw 'Expected SignTool and a 40-character certificate thumbprint'
}
$filePath = (Resolve-Path -LiteralPath $File -ErrorAction Stop).Path
if ($Uninstaller -and ($VerifyOnly -or -not $env:JD_SIGN_UNINSTALLER_RECEIPT)) {
    throw 'The NSIS uninstaller hook requires a fresh signing receipt path'
}

if (-not $VerifyOnly) {
    $arguments = @('sign', '/s', 'My', '/sha1', $thumbprint, '/fd', 'SHA256',
                   '/tr', 'http://timestamp.digicert.com', '/td', 'SHA256')
    if ($env:JD_SIGN_MACHINE_STORE -eq '1') { $arguments += '/sm' }
    & $env:JD_SIGNTOOL @arguments $filePath
    if ($LASTEXITCODE -ne 0) { throw "Signing failed: $filePath" }
}

& $env:JD_SIGNTOOL verify /pa /all /tw $filePath
if ($LASTEXITCODE -ne 0) { throw "Signature or timestamp verification failed: $filePath" }
$signature = Get-AuthenticodeSignature -LiteralPath $filePath
if ($signature.Status -ne 'Valid' -or $signature.SignatureType -ne 'Authenticode' -or
    $signature.SignerCertificate.Thumbprint -ine $thumbprint -or
    $null -eq $signature.TimeStamperCertificate) {
    throw "Unexpected signer or missing timestamp: $filePath"
}
if ($Uninstaller) {
    $receipt = [IO.File]::Open($env:JD_SIGN_UNINSTALLER_RECEIPT, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write)
    try {
        $digest = (Get-FileHash -LiteralPath $filePath -Algorithm SHA256).Hash
        $bytes = [Text.Encoding]::ASCII.GetBytes($digest)
        $receipt.Write($bytes, 0, $bytes.Length)
    } finally {
        $receipt.Dispose()
    }
}
