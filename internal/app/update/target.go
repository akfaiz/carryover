package update

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Method is how the installed program is replaced.
type Method string

const (
	// MethodNone: this install can't replace itself (a folder it can't write,
	// a platform without a release file); the user gets the release page.
	MethodNone Method = ""
	// MethodInstaller runs the Windows setup program over its install.
	MethodInstaller Method = "installer"
	// MethodPortable replaces a Windows exe unpacked from the zip.
	MethodPortable Method = "portable"
	// MethodAppImage replaces the Linux AppImage file.
	MethodAppImage Method = "appimage"
	// MethodBinary replaces a Linux binary installed from the tarball.
	MethodBinary Method = "binary"
	// MethodMacApp replaces the macOS Carryover.app.
	MethodMacApp Method = "macapp"
	// MethodAPK hands an APK to Android's installer, which the app does
	// itself: Apply doesn't.
	MethodAPK Method = "apk"
)

// Target is the installed program to be updated.
type Target struct {
	OS     string // a GOOS value
	Arch   string // a GOARCH value
	ABI    string // Android only: the device's ABI, such as "arm64-v8a"
	Method Method
	// Exe is the program file that is replaced (for MethodMacApp, the one
	// inside the bundle). Not used on Android.
	Exe string
}

// AssetName is the release file of version that updates t; ok is false when
// there is none. The names are those of .github/workflows/release.yml.
func (t Target) AssetName(version string) (name string, ok bool) {
	var suffix string
	switch {
	case t.Method == MethodAPK && t.ABI != "":
		suffix = "android-" + t.ABI + ".apk"
	case t.Method == MethodInstaller:
		suffix = "windows-" + t.Arch + "-setup.exe"
	case t.Method == MethodPortable:
		suffix = "windows-" + t.Arch + ".zip"
	case t.Method == MethodAppImage:
		suffix = "linux-" + t.Arch + ".AppImage"
	case t.Method == MethodBinary:
		suffix = "linux-" + t.Arch + ".tar.gz"
	case t.Method == MethodMacApp:
		suffix = "macos-universal.zip"
	default:
		return "", false
	}
	return "carryover-" + version + "-" + suffix, true
}

// AndroidTarget describes the Android app whose Go core is built for goarch.
// gomobile builds one core per ABI and each APK holds one, so the core's own
// architecture names the APK that is installed.
func AndroidTarget(goarch string) Target {
	abi := map[string]string{"arm64": "arm64-v8a", "amd64": "x86_64", "arm": "armeabi-v7a", "386": "x86"}[goarch]
	return Target{OS: "android", Arch: goarch, ABI: abi, Method: MethodAPK}
}

// DesktopTarget describes the running desktop program, exe being the file
// Steam would start (core.SelfExecutable: the AppImage file, not the one
// inside it).
func DesktopTarget(exe string) Target {
	return Target{
		OS: runtime.GOOS, Arch: runtime.GOARCH, Exe: exe,
		Method: detect(installation{
			goos: runtime.GOOS, goarch: runtime.GOARCH, exe: exe,
			appImage: os.Getenv("APPIMAGE"), installDir: installDir(), writable: writable,
		}),
	}
}

// installation is what detect looks at (all of it a parameter, for tests).
type installation struct {
	goos, goarch, exe    string
	appImage, installDir string // $APPIMAGE; where the Windows installer installed
	writable             func(dir string) bool
}

func detect(in installation) Method {
	if in.exe == "" {
		return MethodNone
	}
	dir := filepath.Dir(in.exe)
	switch in.goos {
	case "windows":
		if in.goarch != "amd64" || !in.writable(dir) {
			return MethodNone
		}
		if in.installDir != "" && strings.EqualFold(filepath.Clean(in.installDir), filepath.Clean(dir)) {
			return MethodInstaller
		}
		return MethodPortable
	case "linux":
		if in.goarch != "amd64" || !in.writable(dir) {
			return MethodNone
		}
		if in.appImage != "" && filepath.Clean(in.appImage) == filepath.Clean(in.exe) {
			return MethodAppImage
		}
		return MethodBinary
	case "darwin":
		// A quarantined app runs from a read-only copy (App Translocation),
		// which writable refuses.
		if bundle := macBundle(in.exe); bundle != "" && in.writable(filepath.Dir(bundle)) {
			return MethodMacApp
		}
	}
	return MethodNone
}

// macBundle is the .app bundle exe is the program of, or "".
func macBundle(exe string) string {
	macos := filepath.Dir(exe)
	contents := filepath.Dir(macos)
	bundle := filepath.Dir(contents)
	if filepath.Base(macos) != "MacOS" || filepath.Base(contents) != "Contents" || !strings.HasSuffix(bundle, ".app") {
		return ""
	}
	return bundle
}

// writable reports whether files can be made in dir, by making one.
func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".carryover-write-*")
	if err != nil {
		return false
	}
	f.Close()
	return os.Remove(f.Name()) == nil
}
