#!/usr/bin/env bash
# Builds carryover-linux-amd64.AppImage from a Linux build of Carryover.
# Linux only (appimagetool); on Windows run it in WSL or on the Steam Deck.
#
#   packaging/appimage/build.sh [path/to/carryover]
#
# The binary defaults to the fyne-cross output
# (fyne-cross/bin/linux-amd64/carryover). The AppImage is written to
# bin/carryover-linux-amd64.AppImage (override with OUT=...).
#
# Needs appimagetool on PATH:
#   https://github.com/AppImage/appimagetool/releases
# Without FUSE (e.g. in a container) run with APPIMAGE_EXTRACT_AND_RUN=1.
#
# UPDATE_INFO=... embeds update information so AppImageUpdate / Gear Lever can
# update the AppImage in place, and writes <out>.zsync next to it (needs
# zsyncmake, from the zsync package). The release workflow sets it.
set -euo pipefail

cd "$(dirname "$0")/../.."
bin="${1:-fyne-cross/bin/linux-amd64/carryover}"
out="${OUT:-bin/carryover-linux-amd64.AppImage}"

[ -f "$bin" ] || { echo "binary not found: $bin (build it first, see README)" >&2; exit 1; }
command -v appimagetool >/dev/null || { echo "appimagetool not found on PATH (see the header of this script)" >&2; exit 1; }

appdir="$(mktemp -d)/Carryover.AppDir"
trap 'rm -rf "$(dirname "$appdir")"' EXIT

mkdir -p "$appdir/usr/bin"
install -m755 "$bin" "$appdir/usr/bin/carryover"
install -m644 packaging/linux/carryover.desktop "$appdir/carryover.desktop"
install -m644 ui/assets/appicon.png "$appdir/carryover.png"
ln -s carryover.png "$appdir/.DirIcon"
ln -s usr/bin/carryover "$appdir/AppRun"

update=()
if [ -n "${UPDATE_INFO:-}" ]; then
  command -v zsyncmake >/dev/null || { echo "UPDATE_INFO is set but zsyncmake is not on PATH (install zsync)" >&2; exit 1; }
  update=(-u "$UPDATE_INFO")
fi

mkdir -p "$(dirname "$out")"
# appimagetool writes the .zsync file into the current directory.
(cd "$(dirname "$out")" && ARCH=x86_64 appimagetool "${update[@]}" "$appdir" "$(basename "$out")")
echo "wrote $out"
