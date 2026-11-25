<#
.SYNOPSIS
  Build the Android app (APKs) on Windows.

.DESCRIPTION
  Builds the Go core with gomobile (android\app\libs\carryover-core.aar, as
  scripts/android-bind.sh does), then the APKs with Gradle, one per ABI, and
  copies them to dist\ (or -OutDir):
    carryover-<version>-android-<abi>.apk          -Release (signed)
    carryover-<version>-android-<abi>-debug.apk    default

  -Release needs the signing key in the environment: CARRYOVER_KEYSTORE (path
  to the keystore), CARRYOVER_KEYSTORE_PASSWORD, CARRYOVER_KEY_ALIAS and, if
  it differs from the keystore's, CARRYOVER_KEY_PASSWORD. Make one with
    keytool -genkeypair -v -keystore carryover.jks -alias carryover -keyalg RSA -keysize 4096 -validity 10000
  (keytool is in Android Studio's jbr\bin) and keep it out of the repository.

  Needs Go, the Android SDK with an NDK (ANDROID_HOME, else
  %LOCALAPPDATA%\Android\Sdk, where Android Studio installs it) and a JDK 17+
  (JAVA_HOME, else Android Studio's bundled jbr, else java on PATH). Like
  windows-package.ps1 it doesn't stamp the version into the Go core: run
  scripts/stamp-version.sh first for that, otherwise the app reports "dev".

.EXAMPLE
  .\scripts\android-build.ps1                                # debug APKs, version from `git describe`
  .\scripts\android-build.ps1 -Release -Version 0.4.0        # signed release APKs
  .\scripts\android-build.ps1 -Targets android/arm64         # devices only (no emulator APK)
  .\scripts\android-build.ps1 -SkipBind                      # Kotlin-only change: reuse the Go core
#>
param(
    [switch]$Release,
    # Version name without the "v" (0.4.0, 0.4.0-beta.1).
    [string]$Version,
    # Added to the version code (0-99, e.g. the CI run number).
    [ValidateRange(0, 99)][int]$Build = 0,
    # gomobile targets; android/amd64 is for the Android emulator and Chromebooks.
    [string]$Targets = 'android/arm64,android/amd64',
    # Reuse android\app\libs\carryover-core.aar.
    [switch]$SkipBind,
    [string]$OutDir
)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
if (-not $OutDir) { $OutDir = Join-Path $root 'dist' }

# Version name and code: x.y.z[-pre] -> x*1000000 + y*10000 + z*100 + build.
if (-not $Version) {
    $Version = (git -C $root describe --tags 2>$null)
    if ($LASTEXITCODE -or -not $Version) { $Version = '0.0.0-dev' }
}
$Version = $Version.Trim() -replace '^v', ''
if ($Version -notmatch '^(\d+)\.(\d+)\.(\d+)') { throw "version must start with x.y.z: $Version" }
$code = [int]$Matches[1] * 1000000 + [int]$Matches[2] * 10000 + [int]$Matches[3] * 100 + $Build
if ($code -lt 1) { $code = 1 }

# Android SDK and NDK.
if (-not $env:ANDROID_HOME) {
    foreach ($d in @($env:ANDROID_SDK_ROOT, (Join-Path $env:LOCALAPPDATA 'Android\Sdk'))) {
        if ($d -and (Test-Path $d)) { $env:ANDROID_HOME = $d; break }
    }
}
if (-not $env:ANDROID_HOME -or -not (Test-Path $env:ANDROID_HOME)) {
    throw 'Android SDK not found: set ANDROID_HOME (Android Studio installs it in %LOCALAPPDATA%\Android\Sdk)'
}
if (-not $env:ANDROID_NDK_HOME) {
    $ndk = Get-ChildItem (Join-Path $env:ANDROID_HOME 'ndk') -Directory -ErrorAction SilentlyContinue |
        Sort-Object { [version]($_.Name -replace '[^\d.].*$', '') } | Select-Object -Last 1
    if (-not $ndk) { throw "no NDK under $env:ANDROID_HOME\ndk: install one in Android Studio (SDK Manager > SDK Tools > NDK)" }
    $env:ANDROID_NDK_HOME = $ndk.FullName
}

# JDK 17+ for Gradle.
if (-not $env:JAVA_HOME) {
    $jbr = Join-Path $env:ProgramFiles 'Android\Android Studio\jbr'
    if (Test-Path (Join-Path $jbr 'bin\java.exe')) { $env:JAVA_HOME = $jbr }
}
$java = if ($env:JAVA_HOME) { Join-Path $env:JAVA_HOME 'bin\java.exe' } else { (Get-Command java -ErrorAction SilentlyContinue).Source }
if (-not $java -or -not (Test-Path $java)) { throw 'no JDK: set JAVA_HOME to a JDK 17 or later (Android Studio''s jbr works)' }
$javaVersion = (& $java -version 2>&1 | Out-String)
if ($javaVersion -notmatch 'version "(\d+)' -or [int]$Matches[1] -lt 17) { throw "Gradle needs JDK 17 or later: $java" }

# ABIs of the APKs: those the Go core is built for.
$abiOf = @{ 'android/arm64' = 'arm64-v8a'; 'android/amd64' = 'x86_64'; 'android/arm' = 'armeabi-v7a'; 'android/386' = 'x86' }
$abis = foreach ($t in ($Targets -split ',')) {
    $t = $t.Trim()
    if (-not $abiOf.ContainsKey($t)) { throw "unsupported target: $t" }
    $abiOf[$t]
}

if ($Release) {
    if (-not $env:CARRYOVER_KEYSTORE) {
        throw '-Release needs the signing key: set CARRYOVER_KEYSTORE, CARRYOVER_KEYSTORE_PASSWORD and CARRYOVER_KEY_ALIAS (see Get-Help .\scripts\android-build.ps1)'
    }
    if (-not (Test-Path $env:CARRYOVER_KEYSTORE)) { throw "keystore not found: $env:CARRYOVER_KEYSTORE" }
    if (-not $env:CARRYOVER_KEYSTORE_PASSWORD -or -not $env:CARRYOVER_KEY_ALIAS) {
        throw 'set CARRYOVER_KEYSTORE_PASSWORD and CARRYOVER_KEY_ALIAS too'
    }
    $env:CARRYOVER_KEYSTORE = (Resolve-Path $env:CARRYOVER_KEYSTORE).Path
}

$variant = if ($Release) { 'release' } else { 'debug' }
Write-Host "Carryover $Version (code $code), $($abis -join ' '), $variant"

Push-Location $root
try {
    $aar = Join-Path $root 'android\app\libs\carryover-core.aar'
    if ($SkipBind) {
        if (-not (Test-Path $aar)) { throw "-SkipBind: $aar doesn't exist yet" }
    } else {
        # gomobile finds gobind on PATH: build both at the versions pinned in go.mod.
        $tools = Join-Path $root 'dist\android-tools'
        New-Item -ItemType Directory -Force $tools | Out-Null
        go build -o "$tools\" golang.org/x/mobile/cmd/gomobile golang.org/x/mobile/cmd/gobind
        if ($LASTEXITCODE) { throw 'building gomobile failed' }
        $env:PATH = "$tools;$env:PATH"
        New-Item -ItemType Directory -Force (Split-Path $aar) | Out-Null
        Write-Host 'Building the Go core (gomobile bind)'
        gomobile bind "-target=$Targets" -androidapi 26 -javapkg com.akfaiz.carryover.bridge `
            -trimpath -ldflags '-s -w' -o $aar ./mobile
        if ($LASTEXITCODE) { throw 'gomobile bind failed' }
    }

    # Gradle reads the SDK path from local.properties, a properties file: '\' is an
    # escape there, so the path is written with '/', and ':' as '\:' (C\:/Users/...).
    $props = Join-Path $root 'android\local.properties'
    if (-not (Test-Path $props)) {
        $sdk = ($env:ANDROID_HOME -replace '\\', '/') -replace ':', '\:'
        Set-Content -Path $props -Value ("sdk.dir=" + $sdk) -Encoding ascii
    }

    $task = if ($Release) { 'assembleRelease' } else { 'assembleDebug' }
    # Gradle updates an existing APK in place and leaves the space of replaced
    # files in it (10 MB and more after a new Go core): start from a fresh one.
    Remove-Item -Recurse -Force (Join-Path $root "android\app\build\outputs\apk\$variant") -ErrorAction SilentlyContinue
    Push-Location (Join-Path $root 'android')
    try {
        & .\gradlew.bat --no-daemon $task "-Pcarryover.versionName=$Version" "-Pcarryover.versionCode=$code" "-Pcarryover.abis=$($abis -join ',')"
        if ($LASTEXITCODE) { throw 'Gradle build failed' }
    } finally {
        Pop-Location
    }

    New-Item -ItemType Directory -Force $OutDir | Out-Null
    $suffix = if ($Release) { '' } else { '-debug' }
    foreach ($a in $abis) {
        $src = Join-Path $root "android\app\build\outputs\apk\$variant\app-$a-$variant.apk"
        if (-not (Test-Path $src)) { throw "Gradle didn't write $src" }
        $dst = Join-Path $OutDir "carryover-$Version-android-$a$suffix.apk"
        Copy-Item -Force $src $dst
        Write-Host ("Wrote {0} ({1:N1} MB)" -f $dst, ((Get-Item $dst).Length / 1MB))
    }
} finally {
    Pop-Location
}
