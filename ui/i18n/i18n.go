// Package i18n translates Carryover's UI text from the embedded catalogs.
//
// Source strings in code are the English text passed to L or N, which is also
// the catalog key, so a missing translation falls back to the English source.
// The package does not depend on the GUI toolkit: it holds its own go-i18n
// localizer, the way Fyne's lang package does. The GUI also hands the same
// catalogs to its toolkit (FS) so the toolkit's own dialogs are translated.
package i18n

import (
	"embed"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"text/template"

	"github.com/jeandeaual/go-locale"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// translations holds one JSON catalog per language (en.json, id.json, ...).
//
//go:embed translations/*.json
var translations embed.FS

// Dir is the directory of the catalogs inside FS.
const Dir = "translations"

// FS returns the embedded catalogs, one <code>.json per language under Dir.
func FS() embed.FS { return translations }

// Catalog returns the catalog of a language code, if there is one.
func Catalog(code string) ([]byte, bool) {
	if code == "" {
		return nil, false
	}
	data, err := translations.ReadFile(Dir + "/" + code + ".json")
	return data, err == nil
}

// Option is a language the user can choose in Settings. Names are in the
// language itself, so they can be found whatever the current one is.
type Option struct{ Code, Name string }

// Options lists the languages with a catalog. Add one here when you add
// translations/<code>.json.
func Options() []Option {
	return []Option{{"en", "English"}, {"id", "Bahasa Indonesia"}}
}

var (
	loadOnce sync.Once
	started  string

	mu        sync.RWMutex
	localizer *goi18n.Localizer
)

// Load sets up the catalogs for the language setting, once per process (the
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

// Init loads every catalog and picks the language: the setting when a catalog
// exists for it, otherwise the one closest to the system locales, otherwise
// English. Use Load outside tests.
func Init(setting string) {
	bundle := goi18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	tags := []language.Tag{language.English} // the first is the fallback
	files, err := translations.ReadDir(Dir)
	if err != nil {
		log.Printf("translations: %v", err)
	}
	for _, f := range files {
		data, err := translations.ReadFile(Dir + "/" + f.Name())
		if err == nil {
			var mf *goi18n.MessageFile
			if mf, err = bundle.ParseMessageFileBytes(data, f.Name()); err == nil {
				tags = append(tags, mf.Tag)
				continue
			}
		}
		log.Printf("translations: %s: %v", f.Name(), err)
	}
	lang := setting
	if _, ok := Catalog(setting); !ok {
		lang = systemLanguage(tags)
	}
	mu.Lock()
	localizer = goi18n.NewLocalizer(bundle, lang)
	mu.Unlock()
}

// systemLanguage is the catalog closest to the user's locales, as Fyne's
// closestSupportedLocale picks it.
func systemLanguage(tags []language.Tag) string {
	locs, err := locale.GetLocales()
	if err != nil || len(locs) == 0 {
		return tags[0].String()
	}
	want := make([]language.Tag, 0, len(locs))
	for _, l := range locs {
		if t, err := language.Parse(l); err == nil {
			want = append(want, t)
		}
	}
	_, i, conf := language.NewMatcher(tags).Match(want...)
	if conf == language.No {
		i = 0
	}
	return tags[i].String()
}

func current() *goi18n.Localizer {
	mu.RLock()
	l := localizer
	mu.RUnlock()
	if l == nil {
		Init("")
		mu.RLock()
		l = localizer
		mu.RUnlock()
	}
	return l
}

// L translates text, filling {{.Name}} placeholders from data[0].
func L(text string, data ...any) string { return localize(text, nil, data) }

// N translates text with plural forms chosen by count ("one", "other", ...),
// filling placeholders from data[0].
func N(text string, count int, data ...any) string { return localize(text, count, data) }

func localize(key string, count any, data []any) string {
	var d0 any
	if len(data) > 0 {
		d0 = data[0]
	}
	ret, err := current().Localize(&goi18n.LocalizeConfig{
		DefaultMessage: &goi18n.Message{ID: key, Other: key},
		PluralCount:    count,
		TemplateData:   d0,
	})
	if err != nil && ret == "" {
		return fallback(key, d0)
	}
	return ret
}

// fallback renders the English source itself, as Fyne's fallbackWithData.
func fallback(key string, data any) string {
	t, err := template.New(key).Parse(key)
	if err != nil {
		return key
	}
	var b strings.Builder
	_ = t.Execute(&b, data)
	return b.String()
}
