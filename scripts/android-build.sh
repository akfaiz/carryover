#!/usr/bin/env bash
# Builds the Android app: the Go core (scripts/android-bind.sh), then the
# APKs with Gradle, one per ABI, copied to dist/ as
#   carryover-<version>-android-<abi>.apk          (--release, signed)
#   carryover-<version>-android-<abi>-debug.apk    (default)
#
# Usage: scripts/android-build.sh [--release] [--version X.Y.Z] [--build N]
#                                 [--targets android/arm64,android/amd64]
#                                 [--skip-bind] [--out DIR]
#
#   --release    release build (shrunk with R8, signed). Needs the signing key
#                in the environment: CARRYOVER_KEYSTORE (path to the keystore),
#                CARRYOVER_KEYSTORE_PASSWORD, CARRYOVER_KEY_ALIAS and, if it
#                differs from the keystore's, CARRYOVER_KEY_PASSWORD. Make one with
#                keytool -genkeypair -v -keystore carryover.jks -alias carryover \
#                        -keyalg RSA -keysize 4096 -validity 10000
#                and keep it out of the repository.
#   --version    the version name (default: `git describe`, else 0.0.0-dev)
#   --build      added to the version code (0-99, e.g. the CI run number)
#   --targets    gomobile targets (default android/arm64,android/amd64;
#                android/arm64 alone for devices only)
#   --skip-bind  reuse android/app/libs/carryover-core.aar (Kotlin-only changes)
#
# Needs Go, the Android SDK with an NDK (ANDROID_HOME, else ~/Android/Sdk or
# ~/Library/Android/sdk) and a JDK 17+ (JAVA_HOME, else java on PATH, else
# Android Studio's bundled one). Like the desktop packaging scripts it doesn't
# stamp the version into the Go core: run scripts/stamp-version.sh first for
# that, otherwise the app reports "dev".
set -euo pipefail
cd "$(dirname "$0")/.."
root=$PWD

release=0 version="" build=0 targets="android/arm64,android/amd64" skip_bind=0 out="$root/dist"
while [ $# -gt 0 ]; do
	case "$1" in
	--release) release=1 ;;
	--version) version=${2:?--version needs a value}; shift ;;
	--build) build=${2:?--build needs a value}; shift ;;
	--targets) targets=${2:?--targets needs a value}; shift ;;
	--skip-bind) skip_bind=1 ;;
	--out) out=${2:?--out needs a value}; shift ;;
	-h | --help) sed -n '2,/^set -euo/p' "$0" | sed '$d' | sed 's/^# \{0,1\}//'; exit 0 ;;
	*) echo "unknown option: $1 (see --help)" >&2; exit 2 ;;
	esac
	shift
done

die() { echo "error: $*" >&2; exit 1; }

# Version name and code: x.y.z[-pre] -> x*1000000 + y*10000 + z*100 + build.
if [ -z "$version" ]; then
	version=$(git describe --tags 2>/dev/null || echo 0.0.0-dev)
fi
version=${version#v}
[[ $version =~ ^([0-9]+)\.([0-9]+)\.([0-9]+) ]] || die "version must start with x.y.z: $version"
major=${BASH_REMATCH[1]} minor=${BASH_REMATCH[2]} patch=${BASH_REMATCH[3]}
[[ $build =~ ^[0-9]+$ ]] && [ "$build" -le 99 ] || die "--build must be 0-99: $build"
code=$((10#$major * 1000000 + 10#$minor * 10000 + 10#$patch * 100 + 10#$build))
[ "$code" -ge 1 ] || code=1

# Android SDK.
if [ -z "${ANDROID_HOME:-}" ]; then
	for d in "${ANDROID_SDK_ROOT:-}" "$HOME/Android/Sdk" "$HOME/Library/Android/sdk"; do
		if [ -n "$d" ] && [ -d "$d" ]; then ANDROID_HOME=$d; break; fi
	done
fi
[ -n "${ANDROID_HOME:-}" ] && [ -d "$ANDROID_HOME" ] || die "Android SDK not found: set ANDROID_HOME (Android Studio installs it)"
export ANDROID_HOME

# JDK 17+ for Gradle.
if [ -z "${JAVA_HOME:-}" ] && ! command -v java >/dev/null; then
	for d in /opt/android-studio/jbr "$HOME/android-studio/jbr" /snap/android-studio/current/jbr \
		"/Applications/Android Studio.app/Contents/jbr/Contents/Home"; do
		if [ -x "$d/bin/java" ]; then export JAVA_HOME=$d; break; fi
	done
fi
java_bin=${JAVA_HOME:+$JAVA_HOME/bin/}java
command -v "$java_bin" >/dev/null || die "no JDK: set JAVA_HOME to a JDK 17 or later (Android Studio's jbr works)"
java_major=$("$java_bin" -version 2>&1 | sed -n 's/.*version "\([0-9]*\).*/\1/p' | head -1)
[ "${java_major:-0}" -ge 17 ] || die "Gradle needs JDK 17 or later, found ${java_major:-unknown} ($java_bin)"

# ABIs of the APKs: those the Go core is built for.
abis=""
IFS=, read -ra list <<<"$targets"
for t in "${list[@]}"; do
	case "$t" in
	android/arm64) a=arm64-v8a ;;
	android/amd64) a=x86_64 ;;
	android/arm) a=armeabi-v7a ;;
	android/386) a=x86 ;;
	*) die "unsupported target: $t" ;;
	esac
	abis=${abis:+$abis,}$a
done

if [ "$release" = 1 ]; then
	[ -n "${CARRYOVER_KEYSTORE:-}" ] || die "--release needs the signing key: set CARRYOVER_KEYSTORE, CARRYOVER_KEYSTORE_PASSWORD and CARRYOVER_KEY_ALIAS (see --help)"
	[ -f "$CARRYOVER_KEYSTORE" ] || die "keystore not found: $CARRYOVER_KEYSTORE"
	[ -n "${CARRYOVER_KEYSTORE_PASSWORD:-}" ] && [ -n "${CARRYOVER_KEY_ALIAS:-}" ] ||
		die "set CARRYOVER_KEYSTORE_PASSWORD and CARRYOVER_KEY_ALIAS too"
	CARRYOVER_KEYSTORE=$(cd "$(dirname "$CARRYOVER_KEYSTORE")" && pwd)/$(basename "$CARRYOVER_KEYSTORE")
	# Gradle runs on the Windows JDK under Git Bash: give it a Windows path.
	if command -v cygpath >/dev/null; then CARRYOVER_KEYSTORE=$(cygpath -m "$CARRYOVER_KEYSTORE"); fi
	export CARRYOVER_KEYSTORE
fi

echo "Carryover $version (code $code), ${abis//,/ }, $([ "$release" = 1 ] && echo release || echo debug)"

if [ "$skip_bind" = 1 ]; then
	[ -f android/app/libs/carryover-core.aar ] || die "--skip-bind: android/app/libs/carryover-core.aar doesn't exist yet"
else
	TARGETS=$targets scripts/android-bind.sh
fi

# Gradle reads the SDK path from local.properties, a properties file: '\' is an
# escape there (a Windows path from Git Bash is written with '/') and ':' is
# written '\:' (C\:/Users/...).
if [ ! -f android/local.properties ]; then
	sdk=$ANDROID_HOME
	if command -v cygpath >/dev/null; then sdk=$(cygpath -m "$sdk"); fi
	colon='\:'
	printf 'sdk.dir=%s\n' "${sdk//:/$colon}" >android/local.properties
fi

if [ "$release" = 1 ]; then task=assembleRelease variant=release; else task=assembleDebug variant=debug; fi
# Gradle updates an existing APK in place and leaves the space of replaced
# files in it (10 MB and more after a new Go core): start from a fresh one.
rm -rf "android/app/build/outputs/apk/$variant"
(cd android && ./gradlew --no-daemon "$task" \
	"-Pcarryover.versionName=$version" "-Pcarryover.versionCode=$code" "-Pcarryover.abis=$abis")

mkdir -p "$out"
suffix=$([ "$release" = 1 ] && echo "" || echo "-debug")
IFS=, read -ra abi_list <<<"$abis"
for a in "${abi_list[@]}"; do
	src=android/app/build/outputs/apk/$variant/app-$a-$variant.apk
	[ -f "$src" ] || die "Gradle didn't write $src"
	dst=$out/carryover-$version-android-$a$suffix.apk
	cp "$src" "$dst"
	echo "Wrote $dst ($(du -h "$dst" | cut -f1))"
done
