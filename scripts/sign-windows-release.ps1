param(
    [Parameter(Mandatory)]
    [string] $File
)

$ErrorActionPreference = "Stop"
if (-not $env:JD_SIGNTOOL -or -not $env:JD_SIGN_CERT -or -not $env:WINDOWS_CERT_PASSWORD) {
    throw "Missing Windows signing configuration"
}

& $env:JD_SIGNTOOL sign /f $env:JD_SIGN_CERT /p $env:WINDOWS_CERT_PASSWORD /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 $File
if ($LASTEXITCODE -ne 0) { throw "Signing failed: $File" }
& $env:JD_SIGNTOOL verify /pa $File
if ($LASTEXITCODE -ne 0) { throw "Signature verification failed: $File" }
