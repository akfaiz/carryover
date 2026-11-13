package ui

import (
	"embed"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
)

// translations holds one JSON catalog per language (en.json, id.json, ...).
// Source strings in code are the English text passed to lang.L, which is also
// the catalog key, so a missing translation falls back to the English source.
//
//go:embed translations/*.json
var translations embed.FS

// languageOption is a language the user can choose in Settings. Names are in
// the language itself, so they can be found whatever the current one is.
type languageOption struct{ code, name string }

// languageOptions lists the languages with a catalog. Add one here when you add
// translations/<code>.json.
func languageOptions() []languageOption {
	return []languageOption{{"en", "English"}, {"id", "Bahasa Indonesia"}}
}

// hasLanguage reports whether a catalog exists for the language code.
func hasLanguage(code string) bool {
	_, err := translations.ReadFile("translations/" + code + ".json")
	return code != "" && err == nil
}

// initTranslations registers the embedded catalogs with Fyne's lang package.
// With setting "" Fyne picks the one closest to the system locale. With a
// language code only that catalog is registered, under the base locale "en":
// Fyne has no way to force a locale, but whatever the system language is, the
// base one is what it falls back to, so the chosen catalog always applies.
// Changing the language therefore takes effect when the app starts.
func initTranslations(setting string) {
	if hasLanguage(setting) {
		data, err := translations.ReadFile("translations/" + setting + ".json")
		if err == nil {
			if err = lang.AddTranslationsForLocale(data, fyne.Locale("en")); err == nil {
				return
			}
		}
		log.Printf("translations: %s: %v", setting, err)
	}
	if err := lang.AddTranslationsFS(translations, "translations"); err != nil {
		log.Printf("translations: %v", err)
	}
}
