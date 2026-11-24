package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// glyphs were icons before the kit had any; they render differently (or not at
// all) per font, so a page draws a kit icon instead.
const glyphs = "📁🗑⟳⚠✓✕▶●＋⋯←▾"

// TestPagesUseTheKitStyle keeps every page on one look: buttons come from
// kit.Btn/IconBtn, sizes from the kit's tokens, and icons from kit.Icon*. Only
// ui/kit and ui/shell may reach for the toolkit's own buttons and raw sizes.
func TestPagesUseTheKitStyle(t *testing.T) {
	fset := token.NewFileSet()
	for _, dir := range []string{"pages", "components", "flows", "prompts"} {
		err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			require.NoError(t, err)
			ast.Inspect(f, func(n ast.Node) bool {
				pos := func() string { return fset.Position(n.Pos()).String() }
				switch n := n.(type) {
				case *ast.BasicLit:
					if n.Kind != token.STRING {
						return true
					}
					s, _ := strconv.Unquote(n.Value)
					if i := strings.IndexAny(s, glyphs); i >= 0 {
						assert.Failf(t, "glyph as icon", "%s: %q: use a kit.Icon*", fset.Position(n.Pos()), s)
					}
				case *ast.CallExpr:
					sel, ok := n.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "ui" {
						switch sel.Sel.Name {
						case "Button", "PrimaryButton", "Checkbox":
							assert.Failf(t, "raw toolkit control", "%s: ui.%s: use kit.Btn, kit.IconBtn or kit.Toggle", pos(), sel.Sel.Name)
						case "TextInput", "TextArea":
							// kit.Field registers it with the gamepad as a text field.
							assert.Failf(t, "raw toolkit control", "%s: ui.%s: use kit.Field or kit.TextArea", pos(), sel.Sel.Name)
						}
					}
					if sel.Sel.Name == "FontSize" && len(n.Args) == 1 {
						if _, lit := n.Args[0].(*ast.BasicLit); lit {
							assert.Failf(t, "literal font size", "%s: use a kit.Text* token or a kit text helper", pos())
						}
					}
				}
				return true
			})
			return nil
		})
		require.NoError(t, err)
	}
}
