#!/usr/bin/env bash
# Builds the Go core (package ./mobile) as an Android library with gomobile:
# android/app/libs/carryover-core.aar, which the Gradle project uses.
#
# Needs the Android SDK (ANDROID_HOME) with an NDK installed (the newest one
# under $ANDROID_HOME/ndk is used unless ANDROID_NDK_HOME is set), and Java.
# TARGETS overrides the ABIs (default android/arm64,android/amd64; amd64 is
# for the Android emulator and Chromebooks).
set -euo pipefail
cd "$(dirname "$0")/.."

: "${ANDROID_HOME:?set ANDROID_HOME to the Android SDK folder}"
if [ -z "${ANDROID_NDK_HOME:-}" ]; then
	ANDROID_NDK_HOME=$(ls -d "$ANDROID_HOME"/ndk/*/ 2>/dev/null | sort -V | tail -1)
	ANDROID_NDK_HOME=${ANDROID_NDK_HOME%/}
	[ -n "$ANDROID_NDK_HOME" ] || { echo "no NDK under $ANDROID_HOME/ndk" >&2; exit 1; }
	export ANDROID_NDK_HOME
fi

# gomobile finds gobind on PATH: build both at the versions pinned in go.mod.
tools="$PWD/dist/android-tools"
mkdir -p "$tools"
go build -o "$tools/" golang.org/x/mobile/cmd/gomobile golang.org/x/mobile/cmd/gobind
export PATH="$tools:$PATH"

out=android/app/libs/carryover-core.aar
mkdir -p "$(dirname "$out")"
gomobile bind \
	-target="${TARGETS:-android/arm64,android/amd64}" \
	-androidapi 26 \
	-javapkg com.akfaiz.carryover.bridge \
	-trimpath -ldflags "-s -w" \
	-o "$out" ./mobile
ls -l "$out"
