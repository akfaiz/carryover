<#
.SYNOPSIS
  Build the Windows release files: the portable zip and the NSIS installer.

.DESCRIPTION
  Writes to dist\ (or -OutDir):
    carryover-<version>-windows-amd64.zip          portable: carryover.exe, the exe from `mygo build`
    carryover-<version>-windows-amd64-setup.exe    installer (packaging\windows\carryover.nsi)

  The release workflow runs this too, so a local build matches the release.
  It does not stamp the version into the program: run scripts/stamp-version.sh
  first for that (and delete internal\app\version\zz_release.go afterwards),
  otherwise the app reports "dev".

  Needs Go and NSIS (`winget install NSIS.NSIS` or `choco install nsis`). No C
  compiler: the GUI is built without cgo.

.EXAMPLE
  .\scripts\windows-package.ps1                     # version from `git describe`
  .\scripts\windows-package.ps1 -Version 0.3.0      # carryover-0.3.0-windows-amd64{.zip,-setup.exe}
  .\scripts\windows-package.ps1 -Version 0.3.0-beta.1
#>
param(
    # Release version without the "v", as used in file names (0.3.0, 0.3.0-beta.1).
    [string]$Version,
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

$makensis = Find-Tool 'makensis' @(
    (Join-Path ${env:ProgramFiles(x86)} 'NSIS\makensis.exe'),
    (Join-Path $env:ProgramFiles 'NSIS\makensis.exe')
)
if (-not $makensis) { throw 'makensis not found: install NSIS (`winget install NSIS.NSIS` or `choco install nsis`)' }

$name = "carryover-$Version-windows-amd64"
New-Item -ItemType Directory -Force $OutDir | Out-Null

Push-Location $root
# mygo.json holds the app's name, identifier and icon; the version is put in for
# this build and the file put back as it was.
$mygoJson = Join-Path $root 'mygo.json'
$mygoJsonBytes = [IO.File]::ReadAllBytes($mygoJson)
(Get-Content -Raw $mygoJson) -replace '"version":\s*"[^"]*"', "`"version`": `"$numeric`"" |
    Set-Content -NoNewline -Encoding utf8 $mygoJson
# The exe is built here and only shipped inside the zip and the installer.
$work = Join-Path $OutDir 'windows-work'
try {
    if (Test-Path $work) { Remove-Item -Recurse -Force $work }
    New-Item -ItemType Directory -Force $work | Out-Null

    Write-Host 'Building carryover.exe'
    # mygo build compiles without cgo and adds the icon, the manifest (per-monitor
    # DPI awareness) and the version info; the zip and installer are made below.
    go tool mygo build -platform windows/amd64 -skip-installer -o (Join-Path $work 'out')
    if ($LASTEXITCODE) { throw 'mygo build failed' }
    # Named carryover.exe, without the version: Steam shortcuts point at the path,
    # so it can be replaced by a newer one under the same name.
    $exe = Join-Path $work 'carryover.exe'
    Move-Item -Force (Join-Path $work 'out\windows-amd64\Carryover.exe') $exe

    $zip = Join-Path $OutDir "$name.zip"
    Write-Host "Writing $name.zip"
    if (Test-Path $zip) { Remove-Item -Force $zip }
    Compress-Archive -CompressionLevel Optimal -Path $exe -DestinationPath $zip

    $setup = Join-Path $OutDir "$name-setup.exe"
    Write-Host "Building $name-setup.exe"
    & $makensis /V2 /INPUTCHARSET UTF8 `
        "/DVERSION=$Version" "/DFILEVERSION=$numeric.0" `
        "/DEXE=$exe" "/DOUTFILE=$setup" `
        (Join-Path $root 'packaging\windows\carryover.nsi')
    if ($LASTEXITCODE) { throw 'makensis failed' }

    Write-Host "Wrote $zip"
    Write-Host "Wrote $setup"
} finally {
    if (Test-Path $work) { Remove-Item -Recurse -Force $work }
    [IO.File]::WriteAllBytes($mygoJson, $mygoJsonBytes)
    Pop-Location
}
