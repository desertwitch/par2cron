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

        # Copy source files into the verify dir so par2.exe can find them
        Copy-Item -Path "$sourcesDir\*" -Destination $verifyDir -Recurse -Force
        Write-Output "Copied source files into $verifyDir"

        # Verify with par2.exe (run from the verify dir, check exit code)
        $par2FileName = "$($bundle.Name).p2c.par2"
        Write-Output "Verifying: $par2FileName"
        Push-Location $verifyDir
        & $par2exe.FullName v -q ".\$par2FileName" 2>&1 | Write-Output
        $exitCode = $LASTEXITCODE
        Pop-Location

        if ($exitCode -ne 0) {
            Write-Output "::error::Verification of $($bundle.Name) failed with exit code $exitCode!"
            $failed = $true
        } else {
            Write-Output "OK: $($bundle.Name) verified successfully (exit code 0)."
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

# --- Clean up par2cmdline download ---
Remove-Item -Path $workDir -Recurse -Force -ErrorAction SilentlyContinue

if ($failed) {
    Write-Error "One or more bundles failed verification."
    exit 1
}

Write-Output "All bundles verified successfully."
