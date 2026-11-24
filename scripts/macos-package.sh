#!/usr/bin/env bash
# Builds the macOS release file: a universal (Apple Silicon + Intel) app bundle
# in a zip.
#
#   scripts/macos-package.sh [version]
#     version  release version without the "v" (default: from `git describe`)
#
# Writes to dist/:
#   Carryover.app                                   the bundle
#   carryover-<version>-macos-universal.zip         the bundle, zipped with ditto
#
# The bundle comes from `mygo build` (go tool mygo), which compiles both halves
# without cgo, makes the Info.plist and the icon, and signs it ad hoc. The
# release workflow runs this too, so a local build matches the release. It does
# not stamp the version into the program: run scripts/stamp-version.sh first
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

name="carryover-$version-macos-universal"
dist="$root/dist"
work="$dist/macos-work"
app="$dist/Carryover.app"
rm -rf "$work" "$app" "$dist/$name.zip"
mkdir -p "$work"

# mygo.json holds the app's name, identifier and icon; the version is put in
# for this build and the file put back as it was.
cp mygo.json "$work/mygo.json"
restore() { cp "$work/mygo.json" "$root/mygo.json"; rm -rf "$work"; }
trap restore EXIT
sed -E -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"$numeric\"/" mygo.json && rm -f mygo.json.bak

export MACOSX_DEPLOYMENT_TARGET=12.0 # what Go itself supports
echo "Building Carryover.app"
go tool mygo build -platform darwin/universal -skip-dmg -skip-notarize -skip-installer -o "$work/out"
mv "$work/out/darwin-universal/Carryover.app" "$app"

# Apple Silicon only runs signed code: sign the whole bundle ad hoc.
codesign --force --deep --sign - "$app"

# ditto, not zip: it keeps the bundle's symlinks, permissions and code signature,
# like the Finder's "Compress", and the Finder unzips it to Carryover.app.
echo "Writing $name.zip"
ditto -c -k --sequesterRsrc --keepParent "$app" "$dist/$name.zip"

echo "Wrote $app"
echo "Wrote $dist/$name.zip"
