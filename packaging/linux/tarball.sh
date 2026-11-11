#!/usr/bin/env bash
# Builds the Linux release tarball: the binary, its .desktop file and icon,
# laid out like an install prefix, plus install.sh.
#
#   packaging/linux/tarball.sh path/to/carryover <name> [outdir]
#
# writes <outdir>/<name>.tar.gz (outdir defaults to dist) holding
#   <name>/bin/carryover
#   <name>/share/applications/carryover.desktop
#   <name>/share/icons/hicolor/scalable/apps/carryover.svg
#   <name>/install.sh, README.md, LICENSE
set -euo pipefail

cd "$(dirname "$0")/../.."
bin="${1:?usage: tarball.sh path/to/carryover <name> [outdir]}"
name="${2:?usage: tarball.sh path/to/carryover <name> [outdir]}"
outdir="${3:-dist}"

[ -f "$bin" ] || { echo "binary not found: $bin" >&2; exit 1; }

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
d="$stage/$name"

install -Dm755 "$bin" "$d/bin/carryover"
install -Dm644 packaging/linux/carryover.desktop "$d/share/applications/carryover.desktop"
install -Dm644 ui/assets/appicon.svg "$d/share/icons/hicolor/scalable/apps/carryover.svg"
install -m755 packaging/linux/install.sh "$d/install.sh"
install -m644 README.md LICENSE "$d/"

mkdir -p "$outdir"
tar -C "$stage" -czf "$outdir/$name.tar.gz" "$name"
echo "wrote $outdir/$name.tar.gz"
