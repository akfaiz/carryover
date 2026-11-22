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

func TestImportConfigDataReplacesAndSaysSo(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, Init(filepath.Join(dir, "rclone.conf")))
	conf := "[imp-nas]\r\ntype = smb\r\nhost = nas.local\r\n\r\n[imp-dav]\r\ntype = webdav\r\nurl = http://example.com\r\n\r\n[imp-ftp]\r\ntype = ftp\r\n"
	res, err := ImportConfigData([]byte(conf))
	require.NoError(t, err)
	assert.Equal(t, []string{"imp-nas", "imp-dav"}, res.Imported, "a file with Windows line endings reads the same")
	assert.Equal(t, []string{"imp-ftp (ftp)"}, res.Skipped)
	assert.Empty(t, res.Replaced, "nothing was there before")

	res, err = ImportConfigData([]byte("[imp-nas]\ntype = smb\nhost = other.local\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"imp-nas"}, res.Imported)
	assert.Equal(t, []string{"imp-nas"}, res.Replaced, "the old settings of imp-nas are gone")
	var names []string
	for _, r := range ListRemotes() {
		names = append(names, r.Name)
	}
	assert.Subset(t, names, []string{"imp-nas", "imp-dav"}) // rclone's config is shared by the tests of this package

	res, err = ImportConfigData([]byte("[bucket]\ntype = s3\n"))
	require.NoError(t, err)
	assert.Empty(t, res.Imported, "nothing of a supported kind")
	assert.Equal(t, []string{"bucket (s3)"}, res.Skipped)
	res, err = ImportConfigData([]byte("not an rclone.conf at all"))
	require.NoError(t, err)
	assert.Empty(t, res.Imported)
	_, err = ImportConfigData([]byte("RCLONE_ENCRYPT_V0:\nabc"))
	require.ErrorContains(t, err, "encrypted")
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
