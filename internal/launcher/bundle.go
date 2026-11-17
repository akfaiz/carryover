package launcher

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

// BundleExe returns the program inside a macOS app bundle (a folder named
// X.app), or path itself when it is not one. A bundle is a folder, so it can't
// be started directly; its program is Contents/MacOS/<CFBundleExecutable>.
// Without a readable Info.plist (binary plists aren't read) it falls back to
// the only file in Contents/MacOS, then to one named after the bundle.
func BundleExe(path string) string {
	if !strings.EqualFold(filepath.Ext(path), ".app") {
		return path
	}
	if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
		return path
	}
	dir := filepath.Join(path, "Contents", "MacOS")
	if name := plistExecutable(filepath.Join(path, "Contents", "Info.plist")); name != "" {
		if bin := filepath.Join(dir, name); isFile(bin) {
			return bin
		}
	}
	if ents, err := os.ReadDir(dir); err == nil {
		var files []string
		for _, e := range ents {
			if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				files = append(files, e.Name())
			}
		}
		if len(files) == 1 {
			return filepath.Join(dir, files[0])
		}
	}
	if bin := filepath.Join(dir, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))); isFile(bin) {
		return bin
	}
	return path
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// plistExecutable reads CFBundleExecutable from an XML property list: the
// <string> right after <key>CFBundleExecutable</key> in the top dictionary.
// It returns "" for a binary plist or when the key is missing.
func plistExecutable(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || bytes.HasPrefix(data, []byte("bplist")) {
		return ""
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	depth := 0 // nesting of <dict>/<array> below <plist>
	wantValue := false
	for {
		tok, err := dec.Token()
		if err != nil { // io.EOF: no such key
			return ""
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "dict", "array":
				depth++
				wantValue = false
			case "key":
				var k string
				if dec.DecodeElement(&k, &t) != nil {
					return ""
				}
				wantValue = depth == 1 && strings.TrimSpace(k) == "CFBundleExecutable"
			case "string":
				var v string
				if dec.DecodeElement(&v, &t) != nil {
					return ""
				}
				if wantValue {
					return filepath.Base(strings.TrimSpace(v))
				}
			default:
				wantValue = false
			}
		case xml.EndElement:
			if t.Name.Local == "dict" || t.Name.Local == "array" {
				depth--
			}
		}
	}
}

// AppBundle returns the app bundle a program inside it belongs to: for
// ".../X.app/Contents/MacOS/x" it returns ".../X.app". Other paths come back
// unchanged. A file picker can't choose a bundle (it shows it as a folder), so
// a picked program inside one is stored as the bundle.
func AppBundle(path string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "MacOS" {
		return path
	}
	contents := filepath.Dir(dir)
	if filepath.Base(contents) != "Contents" {
		return path
	}
	if app := filepath.Dir(contents); strings.EqualFold(filepath.Ext(app), ".app") {
		return app
	}
	return path
}
