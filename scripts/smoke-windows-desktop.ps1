#Requires -Version 7.0
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$Application,
    [ValidateLength(1, 128)][string]$ExpectedVersion,
    [ValidateRange(5, 180)][int]$StartupTimeoutSeconds = 45,
    [ValidateSet('Exit', 'Minimize', 'Tray')][string]$CloseScenario = 'Exit',
    [string]$NativeProbe,
    [switch]$RequireNotificationArea,
    [switch]$VerifyCMSProtocol
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'smoke-windows-session.ps1')

function Stop-ProfileWebviews([string]$Profile) {
    $browsers = Get-CimInstance -ClassName Win32_Process -Filter "Name = 'msedgewebview2.exe'"
    foreach ($browser in $browsers) {
        # The profile contains this run's unique GUID. Leave every other WebView alone.
        if (-not $browser.CommandLine -or
            -not $browser.CommandLine.Contains($Profile, [StringComparison]::OrdinalIgnoreCase)) {
            continue
        }
        try {
            $child = [Diagnostics.Process]::GetProcessById($browser.ProcessId)
        } catch [ArgumentException] {
            continue
        }
        try {
            if ($child.HasExited -or $child.ProcessName -ne 'msedgewebview2' -or
                $browser.CreationDate -isnot [DateTime]) { continue }
            $ageDifference = $child.StartTime.ToUniversalTime() - $browser.CreationDate.ToUniversalTime()
            if ([Math]::Abs($ageDifference.TotalMilliseconds) -ge 1) { continue }
            $child.Kill($true)
            if (-not $child.WaitForExit(5000)) {
                throw "Isolated WebView2 process did not stop: $($browser.ProcessId)"
            }
        } finally {
            $child.Dispose()
        }
    }
}

function Remove-TestProfile([string]$Root) {
    for ($attempt = 1; $attempt -le 20; $attempt++) {
        if (-not (Test-Path -LiteralPath $Root)) { return }
        try {
            Remove-Item -LiteralPath $Root -Recurse -Force -ErrorAction Stop
            return
        } catch {
            if ($attempt -eq 20) { throw }
            Start-Sleep -Milliseconds 250
        }
    }
}

function Invoke-DesktopProbe([string[]]$ProbeArguments) {
    $result = & $probe @ProbeArguments
    if ($LASTEXITCODE -ne 0) { throw "Native probe failed: $($ProbeArguments[0]) ($LASTEXITCODE)" }
    return $result
}

function Wait-WindowState([Diagnostics.Process]$Process, [string]$Description, [scriptblock]$Expected) {
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while ($timer.Elapsed.TotalSeconds -lt 15) {
        if ($Process.HasExited) { throw "Application exited while waiting for $Description ($($Process.ExitCode))" }
        $state = Invoke-DesktopProbe @('inspect', [string]$Process.Id) | ConvertFrom-Json
        if (& $Expected $state) { return $state }
        Start-Sleep -Milliseconds 200
    }
    throw "Timed out waiting for ${Description}. Last window state: $($state | ConvertTo-Json -Compress)"
}

function Send-NativeClose([Diagnostics.Process]$Process) {
    $Process.Refresh()
    if (-not $Process.CloseMainWindow()) { throw 'Unable to send the native window-close request' }
}

function Start-CMSProtocol([Diagnostics.ProcessStartInfo]$AppStart) {
    # ShellExecute uses the caller's environment and cannot use ProcessStartInfo.Environment.
    $keys = @('APPDATA', 'LOCALAPPDATA', 'JINDIDRAMA_DATA_DIR', 'WEBVIEW2_USER_DATA_FOLDER',
        'JINDIDRAMA_LICENSED_MEDIA_DIR', 'JINDIDRAMA_CMS_URL')
    $prior = @{}
    foreach ($key in $keys) {
        $prior[$key] = [Environment]::GetEnvironmentVariable($key)
    }
    try {
        foreach ($key in $keys) {
            [Environment]::SetEnvironmentVariable($key, $AppStart.Environment[$key])
        }
        $launch = [Diagnostics.ProcessStartInfo]::new('jindi://catalog/apple-cms:0123456789abcdef0123456789abcdef:1')
        $launch.UseShellExecute = $true
        $launch.ErrorDialog = $false
        $launch.WorkingDirectory = $AppStart.WorkingDirectory
        return [Diagnostics.Process]::Start($launch)
    } finally {
        foreach ($key in $keys) {
            [Environment]::SetEnvironmentVariable($key, $prior[$key])
        }
    }
}

if (-not $IsWindows) { throw 'This check requires a Windows desktop session' }
$exe = (Resolve-Path -LiteralPath $Application).Path
if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) { throw "Application file does not exist: $exe" }

$probe = $null
if ($NativeProbe) {
    $probe = (Resolve-Path -LiteralPath $NativeProbe).Path
}
if ($RequireNotificationArea -and $CloseScenario -ne 'Tray') {
    throw '-RequireNotificationArea applies only to the Tray scenario'
}
if ($VerifyCMSProtocol -and $CloseScenario -ne 'Minimize') {
    throw '-VerifyCMSProtocol applies only to the Minimize scenario'
}

$root = Join-Path ([IO.Path]::GetTempPath()) ('jindi-desktop-smoke-' + [Guid]::NewGuid().ToString('N'))
[void](New-Item -Path $root -ItemType Directory)
$appData = Join-Path $root 'roaming'
$localData = Join-Path $root 'local'
$webviewData = Join-Path $appData (Split-Path -Leaf $exe)
$dataDir = Join-Path $appData 'JindiDrama'
$sessionDir = Join-Path $dataDir 'logs/sessions'
$app = $null
$relaunch = $null
$failure = $null
$stdout = $null
$stderr = $null
$cleanupErrors = [Collections.Generic.List[System.Management.Automation.ErrorRecord]]::new()
try {
    if (-not $probe) {
        $probe = Join-Path $root 'jindi-desktop-smoke.exe'
        Push-Location (Join-Path $PSScriptRoot '..')
        try {
            & go build -mod=readonly -o $probe ./cmd/windows-desktop-smoke
            if ($LASTEXITCODE -ne 0) { throw 'Could not build the desktop smoke probe' }
        } finally {
            Pop-Location
        }
    }
    foreach ($dir in @($appData, $localData)) {
        [void][IO.Directory]::CreateDirectory($dir)
    }

    if ($CloseScenario -ne 'Exit') {
        [void][IO.Directory]::CreateDirectory($dataDir)
        Invoke-DesktopProbe @('seed', (Join-Path $dataDir 'jindi.db'), $CloseScenario)
    }

    $start = [Diagnostics.ProcessStartInfo]::new($exe)
    $start.WorkingDirectory = Split-Path -LiteralPath $exe
    $start.UseShellExecute = $false
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    $start.Environment['APPDATA'] = $appData
    $start.Environment['JINDIDRAMA_DATA_DIR'] = $dataDir
    $start.Environment['LOCALAPPDATA'] = $localData
    [void]$start.Environment.Remove('WEBVIEW2_USER_DATA_FOLDER')
    [void]$start.Environment.Remove('JINDIDRAMA_LICENSED_MEDIA_DIR')
    [void]$start.Environment.Remove('JINDIDRAMA_CMS_URL')
    $app = [Diagnostics.Process]::Start($start)
    $stdout = $app.StandardOutput.ReadToEndAsync()
    $stderr = $app.StandardError.ReadToEndAsync()
    $timer = [Diagnostics.Stopwatch]::StartNew()
    $ready = $false
    while ($timer.Elapsed.TotalSeconds -lt $StartupTimeoutSeconds) {
        $app.Refresh()
        if ($app.HasExited) { throw "Application exited before readiness (exit code $($app.ExitCode))" }
        if ($app.MainWindowHandle -ne [IntPtr]::Zero -and (Test-PageReady $sessionDir $ExpectedVersion)) {
            $ready = $true
            break
        }
        Start-Sleep -Milliseconds 200
    }
    if (-not $ready) { throw "Timed out waiting for the native window and Wails page readiness ($StartupTimeoutSeconds seconds)" }
    if (-not (Test-Path -LiteralPath $webviewData -PathType Container)) {
        throw "WebView2 did not create its isolated profile: $webviewData"
    }
    Write-Host "Native window and WebView2 page ready (windows/amd64): $exe"

    if ($CloseScenario -eq 'Exit') {
        Send-NativeClose $app
        if (-not $app.WaitForExit(15000)) { throw 'Application did not exit after the window-close request' }
        if ($app.ExitCode -ne 0) { throw "Application exit code: $($app.ExitCode)" }
        if (Get-ChildItem -LiteralPath $sessionDir -Filter 'run-*.session' -File) {
            throw 'Normal exit left a running-session marker'
        }

        $database = Join-Path $dataDir 'jindi.db'
        Invoke-DesktopProbe @('verify-db', $database)
        Invoke-DesktopProbe @('verify-rollback', $exe, $database)
        Write-Host "Packaged SQLite rollback and unchanged source verified: $exe"
        Write-Host "Clean exit, SQLite integrity, and current application schema verified: $exe"
    } else {
        $initial = Wait-WindowState $app 'visible main window' {
            param($state) $state.Main -ne 0 -and $state.Visible -and -not $state.Minimized
        }
        $mainWindow = $initial.Main
        $useTray = $false
        if ($CloseScenario -eq 'Tray') {
            $useTray = Invoke-DesktopProbe @('notification-available') | ConvertFrom-Json
        }
        if ($RequireNotificationArea -and -not $useTray) {
            throw 'The Tray scenario requires a non-elevated Windows Explorer notification area'
        }
        if ($CloseScenario -eq 'Tray' -and -not $useTray) {
            Write-Host 'Notification area unavailable or process elevated. Verifying the tray fallback to minimizing.'
        }

        Send-NativeClose $app
        if ($useTray) {
            [void](Wait-WindowState $app 'hidden main window with tray receiver' {
                param($state) $state.Main -eq $mainWindow -and -not $state.Visible -and $state.Tray -ne 0
            })
            Invoke-DesktopProbe @('activate-tray', [string]$app.Id)
            [void](Wait-WindowState $app 'tray callback restoration and receiver cleanup' {
                param($state) $state.Main -eq $mainWindow -and $state.Visible -and -not $state.Minimized -and $state.Tray -eq 0
            })
            Send-NativeClose $app
            [void](Wait-WindowState $app 'repeated tray hiding' {
                param($state) $state.Main -eq $mainWindow -and -not $state.Visible -and $state.Tray -ne 0
            })
        } else {
            [void](Wait-WindowState $app 'minimized main window' {
                param($state) $state.Main -eq $mainWindow -and $state.Visible -and $state.Minimized -and $state.Tray -eq 0
            })
        }

        $relaunch = [Diagnostics.Process]::Start($start)
        $relaunchOut = $relaunch.StandardOutput.ReadToEndAsync()
        $relaunchErr = $relaunch.StandardError.ReadToEndAsync()
        if (-not $relaunch.WaitForExit(10000)) { throw 'Second launch did not forward to the existing process' }
        if ($relaunch.ExitCode -ne 0) {
            throw "Second launch failed ($($relaunch.ExitCode)): $($relaunchErr.Result) $($relaunchOut.Result)"
        }
        [void](Wait-WindowState $app 'ordinary relaunch restoration of the same window' {
            param($state) $state.Main -eq $mainWindow -and $state.Visible -and -not $state.Minimized -and $state.Tray -eq 0
        })
        Write-Host "Native window close and relaunch verified ($CloseScenario, tray callback exercised: $useTray)."

        if ($VerifyCMSProtocol) {
            $relaunch.Dispose()
            $relaunch = $null
            Send-NativeClose $app
            [void](Wait-WindowState $app 'minimized window before CMS protocol activation' {
                param($state) $state.Main -eq $mainWindow -and $state.Visible -and $state.Minimized
            })
            $relaunch = Start-CMSProtocol $start
            if ($null -eq $relaunch -or -not $relaunch.WaitForExit(10000) -or $relaunch.ExitCode -ne 0) {
                throw 'The installed CMS protocol did not forward to the existing process'
            }
            [void](Wait-WindowState $app 'same window restored by Windows CMS protocol activation' {
                param($state) $state.Main -eq $mainWindow -and $state.Visible -and -not $state.Minimized
            })
            Write-Host 'Installed CMS share protocol restored the original application window.'
        }
    }
} catch {
    $failure = $_
} finally {
    if ($null -ne $relaunch) {
        try {
            if (-not $relaunch.HasExited) {
                $relaunch.Kill($true)
                if (-not $relaunch.WaitForExit(5000)) { throw 'Second test application process did not stop' }
            }
        } catch {
            $cleanupErrors.Add($_)
        } finally {
            $relaunch.Dispose()
        }
    }
    if ($null -ne $app) {
        try {
            if (-not $app.HasExited) {
                $app.Kill($true)
                if (-not $app.WaitForExit(10000)) { throw 'Test application process did not stop' }
            }
        } catch {
            $cleanupErrors.Add($_)
        }
        try {
            Stop-ProfileWebviews $webviewData
        } catch {
            $cleanupErrors.Add($_)
        }
    }
    if ($failure -or $cleanupErrors.Count -gt 0) {
        foreach ($stream in @($stdout, $stderr)) {
            if ($null -eq $stream) { continue }
            try {
                if ($stream.Wait(1000) -and $stream.IsCompletedSuccessfully) {
                    $stream.Result -split "\r?\n" | Select-Object -Last 25 | ForEach-Object { Write-Host $_ }
                }
            } catch {
                Write-Warning "Could not read native process output: $_"
            }
        }
        $log = Join-Path $dataDir 'logs/jindi.log'
        if (Test-Path -LiteralPath $log -PathType Leaf) {
            Get-Content -LiteralPath $log -Tail 25 -ErrorAction Continue | ForEach-Object { Write-Host $_ }
        }
    }
    if ($null -ne $app) { $app.Dispose() }
    try {
        Remove-TestProfile $root
    } catch {
        $cleanupErrors.Add($_)
    }
}
foreach ($cleanupError in $cleanupErrors) { Write-Warning "Smoke cleanup failed: $cleanupError" }
if ($failure) { throw $failure }
if ($cleanupErrors.Count -gt 0) { throw 'The smoke test did not clean up its processes or profile' }
