package editor

import "github.com/akfaiz/carryover/ui/i18n"

// saveKindLabel is the text for a yuzu-family save slot's kind. It lives apart
// from the views because the ViewModel names slots with it too.
func saveKindLabel(kind string) string {
	if kind == saveKindDevice {
		return i18n.L("Device save")
	}
	return i18n.L("Account save")
}

// kindLabel names a kind of save-location suggestion.
func kindLabel(kind string) string {
	switch kind {
	case "native":
		return i18n.L("Linux/macOS native")
	case "proton":
		return i18n.L("Proton prefix")
	case "install":
		return i18n.L("install folder")
	case "steamcloud":
		return i18n.L("Steam Cloud folder")
	case "prefix":
		return i18n.L("Wine prefix")
	}
	return ""
}
