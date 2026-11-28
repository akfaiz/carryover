package mobile

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/msg"
)

// Every exported function that returns an error sends it through wire, so the
// app can translate it.
func TestExportedErrorsAreWired(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() || fn.Type.Results == nil {
				continue
			}
			results := fn.Type.Results.List
			if types.ExprString(results[len(results)-1].Type) != "error" {
				continue
			}
			checked++
			wired := false
			if first, ok := fn.Body.List[0].(*ast.DeferStmt); ok {
				wired = types.ExprString(first.Call) == "wire(&err)"
			}
			assert.True(t, wired, "%s: %s must start with defer wire(&err), err being its named result", name, fn.Name.Name)
		}
	}
	assert.Greater(t, checked, 50)
}

func TestWire(t *testing.T) {
	var err error = msg.LibraryGameNotFound.Err()
	wire(&err)
	payload, ok := strings.CutPrefix(err.Error(), ErrorPrefix)
	require.True(t, ok, err.Error())
	var m msg.Message
	require.NoError(t, json.Unmarshal([]byte(payload), &m))
	assert.Equal(t, msg.LibraryGameNotFound.Code, m.Code)
	assert.Equal(t, "game not found", m.Text)

	plain := errors.New("disk full")
	err = plain
	wire(&err)
	assert.Same(t, plain, err, "an error without a code stays as it is")

	err = nil
	wire(&err)
	assert.NoError(t, err)
}
