#!/usr/bin/env pwsh
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# --- Download latest par2cmdline release ---
Write-Output "Fetching latest par2cmdline release..."
$headers = @{}
if ($env:GITHUB_TOKEN) {
    $headers["Authorization"] = "Bearer $env:GITHUB_TOKEN"
}
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/Parchive/par2cmdline/releases/latest" -Headers $headers
$asset = $release.assets | Where-Object { $_.name -like "*-win-x64.zip" } | Select-Object -First 1
if (-not $asset) {
    Write-Error "No *-win-x64.zip asset found in the latest par2cmdline release."
    exit 1
}

$workDir = Join-Path ([System.IO.Path]::GetTempPath()) "par2cmd-verify-$([System.Guid]::NewGuid().ToString('N').Substring(0,8))"
New-Item -ItemType Directory -Path $workDir -Force | Out-Null

$zipPath = Join-Path $workDir "par2cmdline.zip"
Write-Output "Downloading $($asset.browser_download_url)"
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zipPath
Expand-Archive -Path $zipPath -DestinationPath (Join-Path $workDir "par2cmdline_tool")

$par2exe = Get-ChildItem -Path (Join-Path $workDir "par2cmdline_tool") -Recurse -Filter "par2.exe" | Select-Object -First 1
if (-not $par2exe) {
    Write-Error "Could not find par2.exe in the par2cmdline release."
    exit 1
}
Write-Output "Found par2.exe at: $($par2exe.FullName)"

# --- Bundle definitions ---
# The bundles are copied from testdata/generated rather than regenerated:
# CI runs `make generate` and `make is-clean` first, so the committed files
# are guaranteed byte-identical to what the packer currently produces.
$bundleNames = @("multipar", "par2cmdline", "par2cmdline-turbo", "parpar", "quickpar")

# --- Resolve paths ---
$repoRoot = git rev-parse --show-toplevel
$testdataDir = Join-Path $repoRoot "internal/bundle/testdata"
$sourcesDir = Join-Path $testdataDir "sources"
$generatedDir = Join-Path $testdataDir "generated"
$verifyDir = Join-Path $workDir "verify"

foreach ($d in @($sourcesDir, $generatedDir)) {
    if (-not (Test-Path $d)) {
        Write-Error "Directory not found: $d"
        exit 1
    }
}

# --- Verify, damage, repair each committed bundle ---
$failed = $false

try {
    foreach ($name in $bundleNames) {
        $par2FileName = "$name.p2c.par2"

        Write-Output "============================================"
        Write-Output "Processing: $name"
        Write-Output "============================================"

        # Fresh verify dir so only this bundle is present
        if (Test-Path $verifyDir) {
            Remove-Item -Path $verifyDir -Recurse -Force
        }
        New-Item -ItemType Directory -Path $verifyDir -Force | Out-Null

        $committedBundle = Join-Path $generatedDir $par2FileName
        if (-not (Test-Path $committedBundle)) {
            Write-Output "::error::Committed bundle not found: $committedBundle"
            $failed = $true
            continue
        }

        # Copy the committed bundle and the source files into the verify dir
        Copy-Item -Path $committedBundle -Destination $verifyDir
        Copy-Item -Path "$sourcesDir\*" -Destination $verifyDir -Recurse -Force
        Write-Output "Copied $par2FileName and source files into $verifyDir"

        # Verify with par2.exe (run from the verify dir, check exit code)
        Write-Output "Verifying: $par2FileName"
        Push-Location $verifyDir
        & $par2exe.FullName v -q ".\$par2FileName" 2>&1 | Write-Output
        $exitCode = $LASTEXITCODE
        Pop-Location

        if ($exitCode -ne 0) {
            Write-Output "::error::Verification of $name failed with exit code $exitCode!"
            $failed = $true
        } else {
            Write-Output "OK: $name verified successfully (exit code 0)."
        }

        # --- Repair test ---
        # Record bundle file MD5 before damaging anything
        $bundlePath = Join-Path $verifyDir $par2FileName
        $md5Before = (Get-FileHash -Path $bundlePath -Algorithm MD5).Hash
        Write-Output "Bundle MD5 before repair: $md5Before"

        # Clip one byte from each source file
        Write-Output "Damaging source files for repair test..."
        $sourceFiles = Get-ChildItem -Path $verifyDir -File | Where-Object { $_.Name -ne $par2FileName }
        foreach ($sf in $sourceFiles) {
            $bytes = [System.IO.File]::ReadAllBytes($sf.FullName)
            if ($bytes.Length -gt 0) {
                $trimmed = $bytes[0..($bytes.Length - 2)]
                [System.IO.File]::WriteAllBytes($sf.FullName, $trimmed)
                Write-Output "  Clipped 1 byte from $($sf.Name) ($($bytes.Length) -> $($trimmed.Length))"
            }
        }

        Write-Output "Repairing: $par2FileName"
        Push-Location $verifyDir
        & $par2exe.FullName r -q ".\$par2FileName" 2>&1 | Write-Output
        $repairExitCode = $LASTEXITCODE
        Pop-Location

        if ($repairExitCode -ne 0) {
            Write-Output "::error::Repair of $name failed with exit code $repairExitCode!"
            $failed = $true
        } else {
            Write-Output "OK: $name repaired successfully (exit code 0)."
        }

        # Verify bundle file was not modified during repair
        $md5After = (Get-FileHash -Path $bundlePath -Algorithm MD5).Hash
        Write-Output "Bundle MD5 after repair:  $md5After"
        if ($md5Before -ne $md5After) {
            Write-Output "::error::Bundle file $par2FileName was modified during repair! ($md5Before -> $md5After)"
            $failed = $true
        } else {
            Write-Output "OK: Bundle file unchanged after repair."
        }

        Write-Output ""
    }
} finally {
    # The verify dir lives under the work dir, so this cleans up everything.
    Remove-Item -Path $workDir -Recurse -Force -ErrorAction SilentlyContinue
}

if ($failed) {
    Write-Error "One or more bundles failed verification."
    exit 1
}

Write-Output "All bundles verified successfully."
