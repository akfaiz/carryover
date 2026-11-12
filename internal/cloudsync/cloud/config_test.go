package cloud

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportConfigSkipsUnsupported(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, Init(filepath.Join(dir, "rclone.conf")))
	src := filepath.Join(dir, "other.conf")
	conf := "[gd]\ntype = drive\nscope = drive\n\n[bucket]\ntype = s3\nprovider = AWS\n\n[nas]\ntype = smb\nhost = nas.local\n"
	require.NoError(t, os.WriteFile(src, []byte(conf), 0o644))
	res, err := ImportConfig(src)
	require.NoError(t, err)
	require.Equal(t, []string{"gd", "nas"}, res.Imported)
	require.Equal(t, []string{"bucket (s3)"}, res.Skipped)
	for _, r := range ListRemotes() {
		require.NotEqual(t, "bucket", r.Name, "unsupported remote was imported")
		require.True(t, r.Supported)
	}
}

func TestProvidersMatchSupported(t *testing.T) {
	var got []string
	for _, p := range Providers() {
		got = append(got, p.Type)
	}
	require.Equal(t, Supported, got)
}

func TestToQuestionDone(t *testing.T) {
	for _, out := range []*fs.ConfigOut{nil, {}} {
		assert.True(t, toQuestion(out).Done, "toQuestion(%v)", out)
	}
}

func TestToQuestionOption(t *testing.T) {
	out := &fs.ConfigOut{
		State: "choose",
		Error: "bad value",
		Option: &fs.Option{
			Name:       "region",
			Help:       "  Pick one.\nMore detail.  ",
			Required:   true,
			IsPassword: true,
			Exclusive:  true,
			Default:    true,
			Examples:   fs.OptionExamples{{Value: "eu", Help: "Europe\nand more"}, {Value: "us"}},
		},
	}
	q := toQuestion(out)
	assert.False(t, q.Done)
	assert.Equal(t, "choose", q.State)
	assert.Equal(t, "bad value", q.Error)
	assert.Equal(t, "region", q.Name)
	require.Equal(t, "Pick one.\nMore detail.", q.Help)
	assert.True(t, q.Required)
	assert.True(t, q.IsPassword)
	assert.True(t, q.Exclusive)
	assert.True(t, q.IsBool, "bool default")
	assert.Equal(t, "true", q.Default)
	require.Len(t, q.Examples, 2)
	assert.Equal(t, "Europe", q.Examples[0].Help)
	assert.Equal(t, "us", q.Examples[1].Value)

	out.Option.Default = "eu"
	q = toQuestion(out)
	assert.False(t, q.IsBool, "string default")
	assert.Equal(t, "eu", q.Default)
}

func TestFirstLine(t *testing.T) {
	for in, want := range map[string]string{"": "", "  one  ": "one", "a\nb": "a", "\n a \nb": "a "} {
		assert.Equal(t, want, firstLine(in))
	}
}
