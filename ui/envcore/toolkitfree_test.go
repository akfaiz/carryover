package envcore_test

import (
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const module = "github.com/akfaiz/carryover"

// toolkitFreePackages and toolkitFreeFiles (globs, relative to ui/) must not
// reach a GUI toolkit, even through other packages: they are what a second
// GUI toolkit would reuse unchanged (the ViewModels and their backends, the
// shared state, the translations).
var (
	toolkitFreePackages = []string{"ui/envcore", "ui/store", "ui/i18n", "ui/textutil"}
	toolkitFreeFiles    = []string{
		"pages/*/viewmodel*.go",
		"pages/*/backend.go",
		"pages/cloud/wizard.go",
		"components/emulatorform/viewmodel.go",
	}
)

// toolkit reports whether an import path is (part of) the GUI toolkit.
func toolkit(path string) bool {
	return strings.HasPrefix(path, "github.com/egoist/mygo")
}

// checker walks the module's own packages; anything outside the module only
// counts by its own path (no third-party library imports a GUI toolkit).
type checker struct {
	root string
	ctx  build.Context
	seen map[string]string // package -> the toolkit import it reaches, "" if none
}

func (c *checker) reaches(path string) string {
	if toolkit(path) {
		return path
	}
	if path != module && !strings.HasPrefix(path, module+"/") {
		return ""
	}
	if via, ok := c.seen[path]; ok {
		return via
	}
	c.seen[path] = "" // cycles can't happen in Go, but stay safe
	dir := filepath.Join(c.root, strings.TrimPrefix(strings.TrimPrefix(path, module), "/"))
	pkg, err := c.ctx.ImportDir(dir, 0)
	if err != nil {
		return ""
	}
	for _, imp := range pkg.Imports {
		if via := c.reaches(imp); via != "" {
			c.seen[path] = path + " -> " + via
			return c.seen[path]
		}
	}
	return ""
}

func TestToolkitFree(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	c := &checker{root: root, ctx: build.Default, seen: map[string]string{}}

	for _, p := range toolkitFreePackages {
		assert.Empty(t, c.reaches(module+"/"+p), "%s must not reach a GUI toolkit", p)
	}

	fset := token.NewFileSet()
	var n int
	for _, glob := range toolkitFreeFiles {
		files, err := filepath.Glob(filepath.Join(root, "ui", glob))
		require.NoError(t, err)
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			n++
			af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
			require.NoError(t, err)
			rel, _ := filepath.Rel(root, f)
			for _, imp := range af.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				assert.Empty(t, c.reaches(path), "%s must not reach a GUI toolkit", rel)
			}
		}
	}
	assert.Greater(t, n, 10, "the globs should find the ViewModels")
}
