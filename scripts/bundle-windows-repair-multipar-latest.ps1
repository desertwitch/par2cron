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
$tmpSubdir = "_verify"

$bundles = @(
    @{
        Name    = "multipar"
        GenArgs = @("-dir", "testdata", "-out", "$tmpSubdir/multipar.p2c.par2", "-parse", "multipar/files.par2",
                    "multipar/files.par2", "multipar/files.vol00+7.par2", "multipar/files.vol07+6.par2",
                    "multipar/files.vol13+6.par2")
    },
    @{
        Name    = "par2cmdline"
        GenArgs = @("-dir", "testdata", "-out", "$tmpSubdir/par2cmdline.p2c.par2", "-parse", "par2cmdline/files.par2",
                    "par2cmdline/files.par2", "par2cmdline/files.vol0+1.par2", "par2cmdline/files.vol1+1.par2",
                    "par2cmdline/files.vol2+1.par2")
    },
    @{
        Name    = "par2cmdline-turbo"
        GenArgs = @("-dir", "testdata", "-out", "$tmpSubdir/par2cmdline-turbo.p2c.par2", "-parse", "par2cmdline-turbo/files.par2",
                    "par2cmdline-turbo/files.par2", "par2cmdline-turbo/files.vol0+1.par2",
                    "par2cmdline-turbo/files.vol1+1.par2", "par2cmdline-turbo/files.vol2+1.par2")
    },
    @{
        Name    = "parpar"
        GenArgs = @("-dir", "testdata", "-out", "$tmpSubdir/parpar.p2c.par2", "-parse", "parpar/files.par2",
                    "parpar/files.par2", "parpar/files.vol00+05.par2", "parpar/files.vol05+05.par2",
                    "parpar/files.vol10+03.par2")
    },
    @{
        Name    = "quickpar"
        GenArgs = @("-dir", "testdata", "-out", "$tmpSubdir/quickpar.p2c.par2", "-parse", "quickpar/files.par2",
                    "quickpar/files.par2", "quickpar/files.vol0+1.PAR2", "quickpar/files.vol1+1.PAR2",
                    "quickpar/files.vol2+2.PAR2")
    }
)

# --- Resolve paths ---
$repoRoot = git rev-parse --show-toplevel
$bundleDir = Join-Path $repoRoot "internal/bundle"
$testdataDir = Join-Path $bundleDir "testdata"
$sourcesDir = Join-Path $testdataDir "sources"
$verifyDir = Join-Path $testdataDir $tmpSubdir

if (-not (Test-Path $sourcesDir)) {
    Write-Error "Sources directory not found: $sourcesDir"
    exit 1
}

# --- Generate and verify each bundle ---
$failed = $false

try {
    foreach ($bundle in $bundles) {
        Write-Output "============================================"
        Write-Output "Processing: $($bundle.Name)"
        Write-Output "============================================"

        # Clean the verify dir so only this bundle is present
        if (Test-Path $verifyDir) {
            Remove-Item -Path $verifyDir -Recurse -Force
        }
        New-Item -ItemType Directory -Path $verifyDir -Force | Out-Null

        # Generate the bundle (output goes into testdata/_verify/)
        Write-Output "Running: go run ../../tool/generate-bundle $($bundle.GenArgs -join ' ')"
        Push-Location $bundleDir
        & go run ../../tool/generate-bundle @($bundle.GenArgs)
        $exitCode = $LASTEXITCODE
        Pop-Location

        if ($exitCode -ne 0) {
            Write-Output "::error::generate-bundle failed for $($bundle.Name)"
            $failed = $true
            continue
        }

        # Copy source files into the verify dir so par2j can find them
        Copy-Item -Path "$sourcesDir\*" -Destination $verifyDir -Recurse -Force
        Write-Output "Copied source files into $verifyDir"

        # Verify with par2j (run from the verify dir)
        $par2FileName = "$($bundle.Name).p2c.par2"
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
            Write-Output "::error::Verification of $($bundle.Name): bundle file is Damaged!"
            $failed = $true
        } elseif ($output -match "Useless\s+:\s+""$escapedName""") {
            Write-Output "::error::Verification of $($bundle.Name): bundle file is Useless!"
            $failed = $true
        } elseif ($output -match "Good\s+:\s+""$escapedName""") {
            Write-Output "OK: $($bundle.Name) verified successfully."
        } else {
            Write-Output "::error::Verification of $($bundle.Name): bundle file not found in par2j output!"
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
        $ErrorActionPreference = "Continue"
        $repairOutput = & $par2j.FullName r ".\$par2FileName" 2>&1 | Out-String
        $global:LASTEXITCODE = 0
        $ErrorActionPreference = "Stop"
        Pop-Location
        Write-Output $repairOutput

        if ($repairOutput -match "Repaired successfully") {
            Write-Output "OK: $($bundle.Name) repaired successfully."
        } else {
            Write-Output "::error::Repair of $($bundle.Name) did not report 'Repaired successfully'!"
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
    # Always clean up the temp subdirectory under testdata
    if (Test-Path $verifyDir) {
        Remove-Item -Path $verifyDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Output "Cleaned up $verifyDir"
    }
}

# --- Clean up MultiPar download ---
Remove-Item -Path $workDir -Recurse -Force -ErrorAction SilentlyContinue

if ($failed) {
    Write-Error "One or more bundles failed verification."
    exit 1
}

Write-Output "All bundles verified successfully."
