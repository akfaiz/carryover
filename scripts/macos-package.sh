#!/usr/bin/env bash
# Builds the macOS release file: a universal (Apple Silicon + Intel) app bundle
# in a zip.
#
#   scripts/macos-package.sh [version] [build]
#     version  release version without the "v" (default: from `git describe`)
#     build    CFBundleVersion's build number (default 1; the workflow run number in CI)
#
# Writes to dist/:
#   Carryover.app                                   the bundle
#   carryover-<version>-macos-universal.zip         the bundle, zipped with ditto
#
# The release workflow runs this too, so a local build matches the release. It
# does not stamp the version into the program: run scripts/stamp-version.sh first
# for that (and delete internal/app/version/zz_release.go afterwards), otherwise
# the app reports "dev".
#
# The app is ad-hoc signed only (no Developer ID, not notarized), so Gatekeeper
# asks before its first start (see README.md, "Download").
#
# Needs a Mac with Go and the Xcode command line tools (`xcode-select --install`).
set -euo pipefail

cd "$(dirname "$0")/.."
root=$(pwd)

version="${1:-}"
build="${2:-1}"
if [ -z "$version" ]; then
  version=$(git describe --tags 2>/dev/null || echo 0.0.0-dev)
  version="${version#v}"
fi
# CFBundleShortVersionString takes x.y.z only.
numeric=$(printf '%s' "$version" | sed -nE 's/^([0-9]+\.[0-9]+\.[0-9]+).*/\1/p')
if [ -z "$numeric" ]; then
  echo "version must start with x.y.z: $version" >&2
  exit 1
fi
case "$build" in
  '' | *[!0-9]*) echo "build must be a number: $build" >&2; exit 1 ;;
esac

name="carryover-$version-macos-universal"
dist="$root/dist"
work="$dist/macos-work"
app="$dist/Carryover.app"
rm -rf "$work" "$app" "$dist/$name.zip"
mkdir -p "$work"

# `fyne package` bumps Build in FyneApp.toml; put the file back as it was.
cp FyneApp.toml "$work/FyneApp.toml"
restore() {
  cp "$work/FyneApp.toml" "$root/FyneApp.toml"
  rm -rf "$root/Carryover.app" "$work"
}
trap restore EXIT

export CGO_ENABLED=1
export MACOSX_DEPLOYMENT_TARGET=12.0 # what Go itself supports
for arch in arm64 amd64; do
  echo "Building $arch"
  GOARCH=$arch go build -trimpath -ldflags "-s -w" -o "$work/$arch/carryover" .
done

# The program must stay named "carryover": Steam shortcuts to Carryover itself
# are recognised by that name.
mkdir -p "$work/universal"
lipo -create -output "$work/universal/carryover" "$work/arm64/carryover" "$work/amd64/carryover"

echo "Packaging Carryover.app"
go tool fyne package --os darwin --release --exe "$work/universal/carryover" \
  --name Carryover --app-id com.akfaiz.carryover --app-version "$numeric" --app-build "$build"
mv Carryover.app "$app"

# Apple Silicon only runs signed code: sign the whole bundle ad hoc.
codesign --force --deep --sign - "$app"

# ditto, not zip: it keeps the bundle's symlinks, permissions and code signature,
# like the Finder's "Compress", and the Finder unzips it to Carryover.app.
echo "Writing $name.zip"
ditto -c -k --sequesterRsrc --keepParent "$app" "$dist/$name.zip"

echo "Wrote $app"
echo "Wrote $dist/$name.zip"
