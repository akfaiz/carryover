#!/usr/bin/env bash
# Build and run the Linux version of Carryover in Docker on Linux or macOS.
# Counterpart of linux-docker.ps1 (Windows).
#
#   scripts/linux-docker.sh build          # build the image
#   scripts/linux-docker.sh run [args]     # open the GUI (builds first if needed); args go to carryover
#   scripts/linux-docker.sh shell          # bash inside the image
#   scripts/linux-docker.sh export         # copy the Linux binary to bin/linux/carryover
#   scripts/linux-docker.sh appimage       # build bin/carryover-linux-amd64.AppImage
#
# GUI display:
#   Linux: uses your X11/XWayland session (DISPLAY). Access is granted with `xhost` if it is installed.
#   macOS: install XQuartz, enable "Allow connections from network clients" in its Security
#          settings, restart it, then run this script.
# Data lives in .docker-data/ (git-ignored), never in your real Carryover config.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
image=carryover-linux
data="$root/.docker-data"
cmd="${1:-run}"
[ $# -gt 0 ] && shift

build_image() { docker build -t "$image" "$root"; }
ensure_image() { docker image inspect "$image" >/dev/null 2>&1 || build_image; }

# Fills run_args with the docker run flags that connect the container to the host display.
display_args() {
    mkdir -p "$data"
    run_args=(--rm -it -v "$data:/data" -e LIBGL_ALWAYS_SOFTWARE=1)
    if [ "$(uname -s)" = Darwin ]; then
        command -v xhost >/dev/null || { echo "XQuartz not found (brew install --cask xquartz)" >&2; exit 1; }
        open -a XQuartz
        xhost +localhost >/dev/null
        run_args+=(-e DISPLAY=host.docker.internal:0)
    else
        [ -n "${DISPLAY:-}" ] || { echo "DISPLAY is not set: start an X11 or XWayland session" >&2; exit 1; }
        command -v xhost >/dev/null && xhost +local:docker >/dev/null
        run_args+=(-e "DISPLAY=$DISPLAY" -v /tmp/.X11-unix:/tmp/.X11-unix)
    fi
}

case "$cmd" in
    build) build_image ;;
    run)
        ensure_image
        display_args
        docker run "${run_args[@]}" "$image" "$@"
        ;;
    shell)
        ensure_image
        display_args
        docker run "${run_args[@]}" --entrypoint bash "$image"
        ;;
    export)
        ensure_image
        mkdir -p "$root/bin/linux"
        id="$(docker create "$image")"
        docker cp "$id:/usr/local/bin/carryover" "$root/bin/linux/carryover"
        docker rm "$id" >/dev/null
        echo "wrote $root/bin/linux/carryover"
        ;;
    appimage)
        docker build --target appimage-out --output "type=local,dest=$root/bin" "$root"
        chmod +x "$root/bin/carryover-linux-amd64.AppImage"
        echo "wrote $root/bin/carryover-linux-amd64.AppImage"
        ;;
    *)
        echo "usage: $0 {build|run|shell|export|appimage}" >&2
        exit 2
        ;;
esac
