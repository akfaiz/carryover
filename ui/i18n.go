package ui

import (
	"embed"
	"log"

	"fyne.io/fyne/v2/lang"
)

// translations holds one JSON catalog per language (en.json, id.json, ...).
// Source strings in code are the English text passed to lang.L, which is also
// the catalog key, so a missing translation falls back to the English source.
//
//go:embed translations/*.json
var translations embed.FS

// initTranslations registers the embedded catalogs with Fyne's lang package.
// Fyne then picks the one closest to the system locale.
func initTranslations() {
	if err := lang.AddTranslationsFS(translations, "translations"); err != nil {
		log.Printf("translations: %v", err)
	}
}
