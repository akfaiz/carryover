<#
.SYNOPSIS
  Build and run the Linux version of Carryover in Docker on Windows (GUI via WSLg).

.EXAMPLE
  .\scripts\linux-docker.ps1 build          # build the image
  .\scripts\linux-docker.ps1 run            # open the GUI (builds first if needed)
  .\scripts\linux-docker.ps1 run list       # run a CLI subcommand instead of the GUI
  .\scripts\linux-docker.ps1 shell          # bash inside the image
  .\scripts\linux-docker.ps1 export         # copy the Linux binary to bin\linux\carryover
  .\scripts\linux-docker.ps1 appimage       # build bin\carryover-linux-amd64.AppImage (for Steam Deck / Linux)

  Requires Docker Desktop (WSL2 backend) and Windows 11 or Windows 10 with WSLg.
  Data lives in .docker-data\ (git-ignored), never in your real Carryover config.
#>
param(
    [Parameter(Position = 0)][ValidateSet('build', 'run', 'shell', 'export', 'appimage')][string]$Command = 'run',
    [Parameter(Position = 1, ValueFromRemainingArguments)][string[]]$Args
)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$image = 'carryover-linux'
$data = Join-Path $root '.docker-data'

function Build-Image {
    docker build -t $image $root
    if ($LASTEXITCODE) { throw 'docker build failed' }
}

function Ensure-Image {
    docker image inspect $image *> $null
    if ($LASTEXITCODE) { Build-Image }
}

# WSLg sockets as seen from Docker Desktop's WSL2 VM.
$wslg = '/run/desktop/mnt/host/wslg'
function Run-Args {
    New-Item -ItemType Directory -Force $data | Out-Null
    @(
        '--rm', '-it',
        '-v', "${data}:/data",
        '-v', "${wslg}/.X11-unix:/tmp/.X11-unix",
        '-v', "${wslg}:/mnt/wslg",
        '-e', 'DISPLAY=:0',
        '-e', 'WAYLAND_DISPLAY=wayland-0',
        '-e', 'XDG_RUNTIME_DIR=/mnt/wslg/runtime-dir',
        '-e', 'PULSE_SERVER=/mnt/wslg/PulseServer',
        '-e', 'LIBGL_ALWAYS_SOFTWARE=1'   # software GL works everywhere; drop for GPU via /dev/dxg
    )
}

switch ($Command) {
    'build' { Build-Image }
    'run' {
        Ensure-Image
        docker run @(Run-Args) $image @Args
    }
    'shell' {
        Ensure-Image
        docker run @(Run-Args) --entrypoint bash $image
    }
    'appimage' {
        # Needs no FUSE or Linux host: the Dockerfile packages it and Docker writes it to bin\.
        docker build --target appimage-out --output "type=local,dest=$(Join-Path $root 'bin')" $root
        if ($LASTEXITCODE) { throw 'docker build failed' }
        Write-Host "Wrote $(Join-Path $root 'bin\carryover-linux-amd64.AppImage')"
    }
    'export' {
        Ensure-Image
        $out = Join-Path $root 'bin\linux'
        New-Item -ItemType Directory -Force $out | Out-Null
        $id = docker create $image
        docker cp "${id}:/usr/local/bin/carryover" (Join-Path $out 'carryover')
        docker rm $id | Out-Null
        Write-Host "Wrote $out\carryover"
    }
}
