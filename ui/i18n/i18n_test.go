package i18n

import (
	"encoding/json"
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestPluralForms(t *testing.T) {
	Init("")
	for n, want := range map[int]string{0: "Synced 0 games", 1: "Synced 1 game", 2: "Synced 2 games"} {
		got := N("Synced {{.Count}} game", n, map[string]any{"Count": n})
		assert.Equal(t, want, got, "N(%d)", n)
	}
	assert.Equal(t, "2 games need attention", N("{{.Count}} game needs attention", 2, map[string]any{"Count": 2}))
	assert.Equal(t, "1 game synced.", N("{{.Count}} game synced.", 1, map[string]any{"Count": 1}))
}

// Every language catalog has exactly the keys of en.json, the same
// placeholders in every message, and the plural forms of the keys that have
// them, so a translation can't drop a {{.Name}} or a count.
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
				assert.Equal(t, len(src), len(got), "plural forms of %q", key)
				for form, text := range got {
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
