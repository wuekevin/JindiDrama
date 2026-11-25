param(
    [Parameter(Mandatory)][ValidatePattern('^[0-9A-Fa-f]{40}$')][string]$SigningThumbprint,
    [Parameter(Mandatory)][string]$SignTool,
    [Parameter(Mandatory)][string]$OutputDirectory,
    [string]$BuildRoot = (Split-Path $PSScriptRoot -Parent),
    [switch]$MachineStore
)

$ErrorActionPreference = 'Stop'
if (-not $IsWindows) { throw 'Windows package signing requires a Windows environment' }
$source = (Resolve-Path -LiteralPath $BuildRoot).Path
$target = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputDirectory)
$parent = Split-Path $target -Parent
if (Test-Path -LiteralPath $target) { throw "Signed output already exists: $target" }
$null = [IO.Directory]::CreateDirectory($parent)
$stage = Join-Path $parent ('.jindi-sign-' + [Guid]::NewGuid().ToString('N'))
$bins = Join-Path $stage 'build/bin'
$scripts = Join-Path $stage 'scripts'
$app = Join-Path $bins 'JindiDrama.exe'
$installer = Join-Path $bins 'JindiDrama-amd64-installer.exe'
$uninstallerReceipt = Join-Path $stage 'uninstaller.sha256'
$previous = @{}
foreach ($variable in @('JD_SIGNTOOL', 'JD_SIGN_THUMBPRINT', 'JD_SIGN_MACHINE_STORE', 'JD_SIGN_UNINSTALLER_RECEIPT')) {
    $previous[$variable] = [Environment]::GetEnvironmentVariable($variable)
}
try {
    $env:JD_SIGNTOOL = (Resolve-Path -LiteralPath $SignTool).Path
    $env:JD_SIGN_THUMBPRINT = $SigningThumbprint
    $env:JD_SIGN_MACHINE_STORE = if ($MachineStore) { '1' } else { $null }
    $env:JD_SIGN_UNINSTALLER_RECEIPT = $uninstallerReceipt
    $null = [IO.Directory]::CreateDirectory($bins)
    $null = [IO.Directory]::CreateDirectory($scripts)
    Copy-Item -LiteralPath (Join-Path $source 'build/windows') -Destination (Join-Path $stage 'build/windows') -Recurse
    Copy-Item -LiteralPath (Join-Path $source 'build/bin/JindiDrama.exe') -Destination $app
    $sign = Join-Path $scripts 'sign-windows-release.ps1'
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'sign-windows-release.ps1') -Destination $sign
    & $sign $app
    Push-Location (Join-Path $stage 'build/windows/installer')
    try {
        & makensis '-DARG_WAILS_AMD64_BINARY=..\..\bin\JindiDrama.exe' -DJINDI_SIGN_UNINSTALLER project.nsi
        if ($LASTEXITCODE -ne 0) { throw 'NSIS signed installer rebuild failed' }
    } finally {
        Pop-Location
    }
    if (-not (Test-Path -LiteralPath $uninstallerReceipt -PathType Leaf) -or
        [IO.File]::ReadAllText($uninstallerReceipt) -notmatch '^[0-9A-Fa-f]{64}$') {
        throw 'NSIS did not run the verified uninstaller signing hook'
    }
    & $sign $installer
    # Both signed files move together, without overwriting an existing output.
    [IO.Directory]::Move($bins, $target)
} finally {
    foreach ($variable in $previous.Keys) {
        [Environment]::SetEnvironmentVariable($variable, $previous[$variable])
    }
    if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
}
