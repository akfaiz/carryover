// Package i18n loads Carryover's translation catalogs into Fyne's lang package.
//
// Source strings in code are the English text passed to lang.L, which is also
// the catalog key, so a missing translation falls back to the English source.
package i18n

import (
	"embed"
	"log"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
)

// translations holds one JSON catalog per language (en.json, id.json, ...).
//
//go:embed translations/*.json
var translations embed.FS

// Option is a language the user can choose in Settings. Names are in the
// language itself, so they can be found whatever the current one is.
type Option struct{ Code, Name string }

// Options lists the languages with a catalog. Add one here when you add
// translations/<code>.json.
func Options() []Option {
	return []Option{{"en", "English"}, {"id", "Bahasa Indonesia"}}
}

// has reports whether a catalog exists for the language code.
func has(code string) bool {
	_, err := translations.ReadFile("translations/" + code + ".json")
	return code != "" && err == nil
}

var (
	loadOnce sync.Once
	started  string
)

// Load registers the catalogs for the language setting, once per process (the
// GUI at startup, the session watcher before its notification), and remembers
// the setting as the one the app started with.
func Load(language string) {
	loadOnce.Do(func() {
		started = language
		Init(language)
	})
}

// Started is the language setting the catalogs were loaded with; a different
// setting applies after a restart.
func Started() string { return started }

// Init registers the embedded catalogs with Fyne's lang package. With setting
// "" Fyne picks the one closest to the system locale. With a language code only
// that catalog is registered, under the base locale "en": Fyne has no way to
// force a locale, but whatever the system language is, the base one is what it
// falls back to, so the chosen catalog always applies. Changing the language
// therefore takes effect when the app starts. Use Load outside tests.
func Init(setting string) {
	if has(setting) {
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
