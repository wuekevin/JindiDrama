function Test-PageReady([string]$SessionDirectory, [string]$ExpectedVersion) {
    if ($ExpectedVersion.Length -gt 128) { throw 'Expected version exceeds the application marker limit' }
    if (-not (Test-Path -LiteralPath $SessionDirectory -PathType Container)) {
        return $false
    }
    $share = [IO.FileShare]::ReadWrite -bor [IO.FileShare]::Delete
    foreach ($marker in Get-ChildItem -LiteralPath $SessionDirectory -Filter 'run-*.session' -File) {
        try {
            $file = [IO.File]::Open($marker.FullName, [IO.FileMode]::Open, [IO.FileAccess]::Read, $share)
            $reader = [IO.BinaryReader]::new($file)
            try {
                if ($reader.ReadByte() -ne [int][char]'R') { continue }
                $data = $reader.ReadBytes(512)
            } finally {
                $reader.Dispose()
            }
        } catch [IO.IOException] {
            # A concurrently created or removed marker can be retried.
            continue
        }
        if ($data.Length -eq 0 -or $data.Length -gt 511) {
            throw 'Ready marker has an invalid size'
        }
        $record = [Text.UTF8Encoding]::new($false, $true).GetString($data) | ConvertFrom-Json -AsHashtable -NoEnumerate
        if ($record -isnot [System.Collections.IDictionary] -or
            $record.version -isnot [string] -or [string]::IsNullOrWhiteSpace($record.version) -or
            $record.os -isnot [string] -or $record.arch -isnot [string]) {
            throw 'Ready marker is missing application identity'
        }
        if ($record.os -cne 'windows' -or $record.arch -cne 'amd64') {
            throw "Running app reported $($record.os)/$($record.arch), expected windows/amd64"
        }
        if ($ExpectedVersion -and $record.version -cne $ExpectedVersion) {
            throw "Running app reported $($record.version), expected $ExpectedVersion"
        }
        return $true
    }
    return $false
}
