package ui

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2/lang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTranslationCatalog checks that every literal passed to lang.L / lang.N
// in this package is listed in translations/en.json, the file translators
// copy when adding a language.
func TestTranslationCatalog(t *testing.T) {
	data, err := translations.ReadFile("translations/en.json")
	require.NoError(t, err)
	var catalog map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &catalog))

	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
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
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "lang" || (sel.Sel.Name != "L" && sel.Sel.Name != "N") {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				assert.Fail(t, "lang call needs a string literal", "%s: lang.%s", fset.Position(call.Pos()), sel.Sel.Name)
				return true
			}
			key, _ := strconv.Unquote(lit.Value)
			assert.Contains(t, catalog, key, "%s: missing from translations/en.json", fset.Position(call.Pos()))
			return true
		})
	}
}

func TestPluralForms(t *testing.T) {
	initTranslations("")
	for n, want := range map[int]string{0: "Synced 0 games", 1: "Synced 1 game", 2: "Synced 2 games"} {
		got := lang.N("Synced {{.Count}} game", n, map[string]any{"Count": n})
		assert.Equal(t, want, got, "N(%d)", n)
	}
	assert.Equal(t, "2 games need attention", lang.N("{{.Count}} game needs attention", 2, map[string]any{"Count": 2}))
	assert.Equal(t, "1 game synced.", lang.N("{{.Count}} game synced.", 1, map[string]any{"Count": 1}))
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
	for _, o := range languageOptions() {
		if o.code == "en" {
			continue
		}
		t.Run(o.code, func(t *testing.T) {
			cat := load(o.code)
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
