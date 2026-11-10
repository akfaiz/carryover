package ui

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2/lang"
)

// TestTranslationCatalog checks that every literal passed to lang.L / lang.N
// in this package is listed in translations/en.json, the file translators
// copy when adding a language.
func TestTranslationCatalog(t *testing.T) {
	data, err := translations.ReadFile("translations/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog map[string]json.RawMessage
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
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
				t.Errorf("%s: lang.%s needs a string literal", fset.Position(call.Pos()), sel.Sel.Name)
				return true
			}
			key, _ := strconv.Unquote(lit.Value)
			if _, ok := catalog[key]; !ok {
				t.Errorf("%s: %q is missing from translations/en.json", fset.Position(call.Pos()), key)
			}
			return true
		})
	}
}

func TestPluralForms(t *testing.T) {
	initTranslations()
	for n, want := range map[int]string{0: "Synced 0 games", 1: "Synced 1 game", 2: "Synced 2 games"} {
		got := lang.N("Synced {{.Count}} game", n, map[string]any{"Count": n})
		if got != want {
			t.Errorf("N(%d) = %q, want %q", n, got, want)
		}
	}
}
