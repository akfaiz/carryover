#!/usr/bin/env bash
# Builds PlayRelay-x86_64.AppImage from a Linux build of PlayRelay.
# Linux only (appimagetool); on Windows run it in WSL or on the Steam Deck.
#
#   packaging/appimage/build.sh [path/to/playrelay]
#
# The binary defaults to the fyne-cross output
# (fyne-cross/bin/linux-amd64/playrelay). The AppImage is written to
# bin/PlayRelay-x86_64.AppImage (override with OUT=...).
#
# Needs appimagetool on PATH:
#   https://github.com/AppImage/appimagetool/releases
# Without FUSE (e.g. in a container) run with APPIMAGE_EXTRACT_AND_RUN=1.
set -euo pipefail

cd "$(dirname "$0")/../.."
bin="${1:-fyne-cross/bin/linux-amd64/playrelay}"
out="${OUT:-bin/PlayRelay-x86_64.AppImage}"

[ -f "$bin" ] || { echo "binary not found: $bin (build it first, see README)" >&2; exit 1; }
command -v appimagetool >/dev/null || { echo "appimagetool not found on PATH (see the header of this script)" >&2; exit 1; }

appdir="$(mktemp -d)/PlayRelay.AppDir"
trap 'rm -rf "$(dirname "$appdir")"' EXIT

mkdir -p "$appdir/usr/bin"
install -m755 "$bin" "$appdir/usr/bin/playrelay"
install -m644 packaging/appimage/playrelay.desktop "$appdir/playrelay.desktop"
install -m644 ui/assets/appicon.png "$appdir/playrelay.png"
ln -s playrelay.png "$appdir/.DirIcon"
ln -s usr/bin/playrelay "$appdir/AppRun"

mkdir -p "$(dirname "$out")"
ARCH=x86_64 appimagetool "$appdir" "$out"
echo "wrote $out"
