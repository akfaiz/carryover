package pathvar

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/saves/ryujinx"
)

func TestRyujinxToken(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "bis", "system", "save", "8000000000000000", "0")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	entries := []ryujinx.Entry{
		{ProgramID: 0x0100152000022000, Type: 1, SaveDataID: 3},
		{ProgramID: 0x0100152000022000, Type: 3, SaveDataID: 4},
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "imkvdb.arc"), ryujinx.EncodeIndex(entries), 0o644))
	ctx := Context{RyujinxDir: root}

	save := func(id string) string { return filepath.Join(root, "bis", "user", "save", id, "0") }
	got, err := ctx.Expand("{ryujinx:0100152000022000:account}")
	require.NoError(t, err)
	assert.Equal(t, save("0000000000000003"), got)
	got, err = ctx.Expand("{ryujinx:0100152000022000}")
	require.NoError(t, err)
	assert.Equal(t, save("0000000000000003"), got, "account is the default kind")
	got, err = ctx.Expand("{ryujinx:0100152000022000:Device}")
	require.NoError(t, err)
	assert.Equal(t, save("0000000000000004"), got)

	_, err = ctx.Expand("{ryujinx:0100000000000001:account}")
	assert.ErrorContains(t, err, "start the game once in Ryujinx")
	_, err = ctx.Expand("{ryujinx:bad}")
	assert.Error(t, err)
	_, err = (Context{RyujinxDir: t.TempDir()}).Expand("{ryujinx:0100152000022000}")
	assert.Error(t, err, "no Ryujinx data in that folder")
}
