<#
.SYNOPSIS
  Build the Windows release files: the portable zip and the NSIS installer.

.DESCRIPTION
  Writes to dist\ (or -OutDir):
    carryover-<version>-windows-amd64.zip          portable: carryover.exe, the exe from `fyne package`
    carryover-<version>-windows-amd64-setup.exe    installer (packaging\windows\carryover.nsi)

  The release workflow runs this too, so a local build matches the release.
  It does not stamp the version into the program: run scripts/stamp-version.sh
  first for that (and delete internal\app\version\zz_release.go afterwards),
  otherwise the app reports "dev".

  Needs Go, gcc on PATH (MSYS2 UCRT64; C:\msys64\ucrt64\bin is tried too) and
  NSIS (`winget install NSIS.NSIS` or `choco install nsis`).

.EXAMPLE
  .\scripts\windows-package.ps1                     # version from `git describe`
  .\scripts\windows-package.ps1 -Version 0.3.0      # carryover-0.3.0-windows-amd64{.zip,-setup.exe}
  .\scripts\windows-package.ps1 -Version 0.3.0-rc1 -Build 42
#>
param(
    # Release version without the "v", as used in file names (0.3.0, 0.3.0-rc1).
    [string]$Version,
    # Fourth part of the exe's file version (the workflow run number in CI).
    [int]$Build = 1,
    [string]$OutDir
)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
if (-not $OutDir) { $OutDir = Join-Path $root 'dist' }

if (-not $Version) {
    $Version = (git -C $root describe --tags 2>$null)
    if ($LASTEXITCODE -or -not $Version) { $Version = '0.0.0-dev' }
    $Version = $Version.Trim() -replace '^v', ''
}
# The exe's version info takes x.y.z only.
if ($Version -notmatch '^(\d+)\.(\d+)\.(\d+)') { throw "version must start with x.y.z: $Version" }
$numeric = $Matches[0]

function Find-Tool([string]$name, [string[]]$candidates) {
    $cmd = Get-Command $name -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    foreach ($c in $candidates) { if ($c -and (Test-Path $c)) { return $c } }
    return $null
}

if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
    if (Test-Path 'C:\msys64\ucrt64\bin\gcc.exe') {
        $env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"
    } else {
        throw 'gcc not found: install MSYS2 and run `pacman -S mingw-w64-ucrt-x86_64-gcc`, then add C:\msys64\ucrt64\bin to PATH'
    }
}
$makensis = Find-Tool 'makensis' @(
    (Join-Path ${env:ProgramFiles(x86)} 'NSIS\makensis.exe'),
    (Join-Path $env:ProgramFiles 'NSIS\makensis.exe')
)
if (-not $makensis) { throw 'makensis not found: install NSIS (`winget install NSIS.NSIS` or `choco install nsis`)' }

$name = "carryover-$Version-windows-amd64"
New-Item -ItemType Directory -Force $OutDir | Out-Null

Push-Location $root
# `fyne package` bumps Build in FyneApp.toml; put the file back as it was.
$fyneApp = Join-Path $root 'FyneApp.toml'
$fyneAppBytes = [IO.File]::ReadAllBytes($fyneApp)
$env:CGO_ENABLED = '1'
# The exe is built here and only shipped inside the zip and the installer.
$work = Join-Path $OutDir 'windows-work'
try {
    if (Test-Path $work) { Remove-Item -Recurse -Force $work }
    New-Item -ItemType Directory -Force $work | Out-Null

    Write-Host 'Building carryover.exe'
    go tool fyne package --os windows --release --name carryover --app-version $numeric --app-build $Build
    if ($LASTEXITCODE) { throw 'fyne package failed' }
    # Named carryover.exe, without the version: Steam shortcuts point at the path,
    # so it can be replaced by a newer one under the same name.
    $exe = Join-Path $work 'carryover.exe'
    Move-Item -Force (Join-Path $root 'carryover.exe') $exe

    $zip = Join-Path $OutDir "$name.zip"
    Write-Host "Writing $name.zip"
    if (Test-Path $zip) { Remove-Item -Force $zip }
    Compress-Archive -CompressionLevel Optimal -Path $exe -DestinationPath $zip

    $setup = Join-Path $OutDir "$name-setup.exe"
    Write-Host "Building $name-setup.exe"
    & $makensis /V2 /INPUTCHARSET UTF8 `
        "/DVERSION=$Version" "/DFILEVERSION=$numeric.$Build" `
        "/DEXE=$exe" "/DOUTFILE=$setup" `
        (Join-Path $root 'packaging\windows\carryover.nsi')
    if ($LASTEXITCODE) { throw 'makensis failed' }

    Write-Host "Wrote $zip"
    Write-Host "Wrote $setup"
} finally {
    if (Test-Path $work) { Remove-Item -Recurse -Force $work }
    [IO.File]::WriteAllBytes($fyneApp, $fyneAppBytes)
    Pop-Location
}
