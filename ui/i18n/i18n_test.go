package i18n

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/akfaiz/carryover/internal/app/msg"
)

// TestTranslationCatalog checks that every literal passed to i18n.L / i18n.N
// under ui/ is listed in translations/en.json, the file translators
// copy when adding a language.
func TestTranslationCatalog(t *testing.T) {
	data, err := translations.ReadFile("translations/en.json")
	require.NoError(t, err)
	var catalog map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &catalog))

	fset := token.NewFileSet()
	err = filepath.WalkDir("..", func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "i18n" || (sel.Sel.Name != "L" && sel.Sel.Name != "N") {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				assert.Fail(t, "i18n call needs a string literal", "%s: i18n.%s", fset.Position(call.Pos()), sel.Sel.Name)
				return true
			}
			key, _ := strconv.Unquote(lit.Value)
			assert.Contains(t, catalog, key, "%s: missing from translations/en.json", fset.Position(call.Pos()))
			return true
		})
		return nil
	})
	require.NoError(t, err)
}

// Every message of the core is listed in en.json under its English template,
// so each language catalog translates it (TestLanguageCatalogsMatchEnglish).
func TestCoreMessagesAreInTheCatalog(t *testing.T) {
	data, err := translations.ReadFile("translations/en.json")
	require.NoError(t, err)
	var catalog map[string]any
	require.NoError(t, json.Unmarshal(data, &catalog))

	for _, d := range msg.Defs() {
		var want any = d.Other
		if d.One != "" {
			want = map[string]any{"one": d.One, "other": d.Other}
		}
		assert.Equal(t, want, catalog[d.Other], "%s in translations/en.json", d.Code)
	}
}

func TestCoreMessagesAreTranslated(t *testing.T) {
	Init("id")
	t.Cleanup(func() { Init("en") })

	assert.Equal(t, "Mengunggah save…", Msg(msg.SessionUploading.Msg()))
	assert.Equal(t, "Mengunggah 2 file (saves)", Msg(msg.SyncUploading.Msg(msg.CountArg, 2, "Slot", "saves")))
	assert.Equal(t, "Belum ada path save untuk Saves, file eksekusi game",
		Msg(msg.SetupMissing.Msg("Items", msg.List{msg.NeedSavePath.Msg("Slot", "Saves"), msg.NeedGameExe.Msg()})))
	// A translated explanation in front of another program's own words.
	err := msg.SessionUploadFailed.Wrap(msg.SyncUploadFailed.Wrap(errors.New("quota exceeded"), "Slot", "saves"))
	assert.Equal(t, "Game ditutup, tetapi unggahan gagal: tidak dapat mengunggah saves: quota exceeded", Err(err))
	// fmt.Errorf around or in front of a message keeps what has a code translated.
	assert.Equal(t, "step 2: tidak ada koneksi internet", Err(fmt.Errorf("step 2: %w", msg.CloudOffline.Err())))
	assert.Equal(t, "tidak ada koneksi internet: dial tcp: timeout", Err(fmt.Errorf("%w: %w", msg.CloudOffline.Err(), errors.New("dial tcp: timeout"))))
	assert.Equal(t, "disk full", Err(errors.New("disk full")))
	assert.Empty(t, Err(nil))
	// A newer core's code this catalog doesn't know shows as its English.
	assert.Equal(t, "Something new", Msg(msg.Message{Code: "new.one", Text: "Something new"}))
}

func TestPluralForms(t *testing.T) {
	Init("")
	for n, want := range map[int]string{0: "Synced 0 games", 1: "Synced 1 game", 2: "Synced 2 games"} {
		got := N("Synced {{.Count}} game", n, map[string]any{"Count": n})
		assert.Equal(t, want, got, "N(%d)", n)
	}
	assert.Equal(t, "2 games need attention", N("{{.Count}} game needs attention", 2, map[string]any{"Count": 2}))
	assert.Equal(t, "1 game synced.", N("{{.Count}} game synced.", 1, map[string]any{"Count": 1}))
}

// Every plural resolves in its own language for any count. Which forms that
// takes is the language's rule (Indonesian has "other", Spanish also "one" and
// "many"), so it is asked of go-i18n with only that catalog loaded: a missing
// form is an error here, where the app would show the English text.
func TestPluralsCoverEachLanguagesForms(t *testing.T) {
	// Counts that reach each form of the usual rules: one, few, many (also
	// the million of the Romance languages) and other.
	counts := []int{0, 1, 2, 5, 11, 21, 22, 25, 100, 101, 1_000_000}
	for _, o := range Options() {
		t.Run(o.Code, func(t *testing.T) {
			data, ok := Catalog(o.Code)
			require.True(t, ok)
			var cat map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data, &cat))
			bundle := goi18n.NewBundle(language.MustParse(o.Code))
			bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
			_, err := bundle.ParseMessageFileBytes(data, o.Code+".json")
			require.NoError(t, err)
			l := goi18n.NewLocalizer(bundle, o.Code)
			for key, raw := range cat {
				if !strings.HasPrefix(string(raw), "{") {
					continue // not a plural
				}
				for _, n := range counts {
					_, err := l.Localize(&goi18n.LocalizeConfig{MessageID: key, PluralCount: n, TemplateData: map[string]any{}})
					if !assert.NoError(t, err, "%q for %d", key, n) {
						break
					}
				}
			}
		})
	}
}

// Every language shows its own text, whether it is chosen in the settings or
// the system's: a regional locale gets the language's catalog.
func TestEveryLanguageApplies(t *testing.T) {
	saved := systemLocales
	t.Cleanup(func() {
		systemLocales = saved
		Init("en")
	})
	cancel := func(code string) string {
		data, ok := Catalog(code)
		require.True(t, ok, code)
		var cat map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &cat))
		var s string
		require.NoError(t, json.Unmarshal(cat["Cancel"], &s))
		return s
	}

	systemLocales = func() ([]string, error) { return []string{"en-US"}, nil }
	for _, o := range Options() {
		Init(o.Code)
		assert.Equal(t, cancel(o.Code), L("Cancel"), "chosen %q", o.Code)
	}
	for system, code := range map[string]string{
		"en-GB": "en", "es-ES": "es", "es-MX": "es", "es-419": "es",
		"id-ID": "id", "pt-BR": "pt-br", "pt-PT": "pt-br", "fr-FR": "en",
	} {
		systemLocales = func() ([]string, error) { return []string{system}, nil }
		Init("")
		assert.Equal(t, cancel(code), L("Cancel"), "system %q", system)
	}
}

// A language's code is its catalog's file name, and the settings save it in
// lower case.
func TestOptionCodesAreLowerCase(t *testing.T) {
	for _, o := range Options() {
		assert.Equal(t, strings.ToLower(o.Code), o.Code)
		_, ok := Catalog(o.Code)
		assert.True(t, ok, "no catalog for %q", o.Code)
	}
}

// Every language catalog has exactly the keys of en.json, the same
// placeholders in every message, and plurals where English has them, so a
// translation can't drop a {{.Name}} or a count.
func TestLanguageCatalogsMatchEnglish(t *testing.T) {
	load := func(name string) map[string]json.RawMessage {
		data, err := translations.ReadFile("translations/" + name + ".json")
		require.NoError(t, err)
		var m map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &m))
		return m
	}
	// forms returns the texts of a catalog value: one string, or one per plural form.
	forms := func(raw json.RawMessage) map[string]string {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return map[string]string{"": s}
		}
		var m map[string]string
		require.NoError(t, json.Unmarshal(raw, &m))
		return m
	}
	placeholders := func(s string) []string {
		out := regexp.MustCompile(`\{\{\.\w+\}\}|\{\w+(:\w+)?\}`).FindAllString(s, -1)
		slices.Sort(out)
		return slices.Compact(out)
	}
	en := load("en")
	for _, o := range Options() {
		if o.Code == "en" {
			continue
		}
		t.Run(o.Code, func(t *testing.T) {
			cat := load(o.Code)
			for key, raw := range en {
				tr, ok := cat[key]
				if !assert.True(t, ok, "missing: %q", key) {
					continue
				}
				src, got := forms(raw), forms(tr)
				_, single := src[""]
				_, translatedSingle := got[""]
				assert.Equal(t, single, translatedSingle, "plural or not: %q", key)
				if !single {
					assert.Contains(t, got, "other", "plural %q", key)
				}
				for form, text := range got {
					if !single {
						assert.Contains(t, []string{"zero", "one", "two", "few", "many", "other"}, form, "plural form of %q", key)
					}
					assert.NotEmpty(t, text, "%q [%s]", key, form)
					// A plural message is compared with the English "other" form: an
					// English singular may leave the count out ("the last backup").
					want := src[""]
					if form != "" {
						want = src["other"]
					}
					assert.Equal(t, placeholders(want), placeholders(text), "placeholders of %q [%s]", key, form)
				}
			}
			for key := range cat {
				assert.Contains(t, en, key, "not in en.json: %q", key)
			}
		})
	}
}

func TestLoadRemembersTheStartingLanguage(t *testing.T) {
	Load("id")
	assert.Equal(t, "id", Started())
	Load("en") // once per process: the first setting stays
	assert.Equal(t, "id", Started())
}

func TestInitFallsBackToAllCatalogs(t *testing.T) {
	Init("") // no language: the system's closest catalog
	Init("xx")
	Init("en")
	assert.Equal(t, "Cancel", L("Cancel"))
}

// A chosen language applies whatever the system locale is.
func TestInitChosenLanguage(t *testing.T) {
	t.Cleanup(func() { Init("en") })
	data, ok := Catalog("id")
	require.True(t, ok)
	var id map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &id))
	var want string
	require.NoError(t, json.Unmarshal(id["Cancel"], &want))
	require.NotEqual(t, "Cancel", want, "id.json should translate Cancel")

	Init("id")
	assert.Equal(t, want, L("Cancel"))
	Init("en")
	assert.Equal(t, "Cancel", L("Cancel"))
}

// A text with no catalog entry is its own English source, placeholders filled.
func TestMissingKeyFallsBackToSource(t *testing.T) {
	Init("id")
	t.Cleanup(func() { Init("en") })
	assert.Equal(t, "Hello Ada", L("Hello {{.Name}}", map[string]any{"Name": "Ada"}))
	assert.Equal(t, "3 things", N("{{.Count}} things", 3, map[string]any{"Count": 3}))
}
