package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInfo(t *testing.T) {
	cases := []struct {
		info        Info
		str, url    string
		wantRelease bool
	}{
		{Info{Version: "0.2.0", Commit: "abc1234"}, "0.2.0 (abc1234)", RepoURL + "/releases/tag/v0.2.0", true},
		{Info{Version: "0.2.0-rc1"}, "0.2.0-rc1", RepoURL + "/releases/tag/v0.2.0-rc1", true},
		{Info{Version: "dev", Commit: "abc1234", Modified: true}, "dev (abc1234, modified)", RepoURL + "/releases", false},
		{Info{Version: "dev"}, "dev", RepoURL + "/releases", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.str, c.info.String())
		assert.Equal(t, c.url, c.info.ReleaseURL())
		assert.Equal(t, c.wantRelease, c.info.IsRelease(), "%q IsRelease()", c.info.Version)
	}
}

func TestGetFromLinkerValues(t *testing.T) {
	defer func(v, c string) { Version, Commit = v, c }(Version, Commit)

	Version, Commit = "v0.2.0", "0123456789abcdef"
	got := Get()
	assert.Equal(t, "0.2.0", got.Version)
	assert.Equal(t, "0123456", got.Commit)
	assert.False(t, got.Modified)

	Version, Commit = "", "abc"
	assert.Equal(t, "dev", Get().Version, "empty version")
}
