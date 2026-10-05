#!/usr/bin/env pwsh
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# --- Download latest MultiPar release ---
Write-Output "Fetching latest MultiPar release..."
$headers = @{}
if ($env:GITHUB_TOKEN) {
    $headers["Authorization"] = "Bearer $env:GITHUB_TOKEN"
}
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/Yutaka-Sawada/MultiPar/releases/latest" -Headers $headers
$asset = $release.assets | Where-Object { $_.name -like "*.zip" } | Select-Object -First 1
if (-not $asset) {
    Write-Error "No .zip asset found in the latest MultiPar release."
    exit 1
}

$workDir = Join-Path ([System.IO.Path]::GetTempPath()) "par2-verify-$([System.Guid]::NewGuid().ToString('N').Substring(0,8))"
New-Item -ItemType Directory -Path $workDir -Force | Out-Null

$zipPath = Join-Path $workDir "multipar.zip"
Write-Output "Downloading $($asset.browser_download_url)"
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zipPath
Expand-Archive -Path $zipPath -DestinationPath (Join-Path $workDir "multipar_tool")

$par2j = Get-ChildItem -Path (Join-Path $workDir "multipar_tool") -Recurse -Filter "par2j*.exe" | Select-Object -First 1
if (-not $par2j) {
    Write-Error "Could not find par2j*.exe in the MultiPar release."
    exit 1
}
Write-Output "Found par2j at: $($par2j.FullName)"

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

        # Verify with par2j (run from the verify dir)
        Write-Output "Verifying: $par2FileName"
        Push-Location $verifyDir
        $output = & $par2j.FullName v ".\$par2FileName" 2>&1 | Out-String
        Pop-Location
        Write-Output $output

        # par2j prints one of these per file:
        #   <packets> <found> Good     : "<file>"
        #   <packets> <found> Damaged  : "<file>"
        #         0     0 Useless  : "<file>"
        # We need to find the line for our bundle file and confirm it says Good.
        $escapedName = [regex]::Escape($par2FileName)
        if ($output -match "Damaged\s+:\s+""$escapedName""") {
            Write-Output "::error::Verification of ${name}: bundle file is Damaged!"
            $failed = $true
        } elseif ($output -match "Useless\s+:\s+""$escapedName""") {
            Write-Output "::error::Verification of ${name}: bundle file is Useless!"
            $failed = $true
        } elseif ($output -match "Good\s+:\s+""$escapedName""") {
            Write-Output "OK: $name verified successfully."
        } else {
            Write-Output "::error::Verification of ${name}: bundle file not found in par2j output!"
            $failed = $true
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
        $repairOutput = & $par2j.FullName r ".\$par2FileName" 2>&1 | Out-String
        $global:LASTEXITCODE = 0 # Above exits with unclean exit code even on success
        Pop-Location
        Write-Output $repairOutput

        if ($repairOutput -match "Repaired successfully") {
            Write-Output "OK: $name repaired successfully."
        } else {
            Write-Output "::error::Repair of $name did not report 'Repaired successfully'!"
            $failed = $true
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
