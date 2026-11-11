package version

import "testing"

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
		if got := c.info.String(); got != c.str {
			t.Errorf("String() = %q, want %q", got, c.str)
		}
		if got := c.info.ReleaseURL(); got != c.url {
			t.Errorf("ReleaseURL() = %q, want %q", got, c.url)
		}
		if got := c.info.IsRelease(); got != c.wantRelease {
			t.Errorf("%q IsRelease() = %v", c.info.Version, got)
		}
	}
}

func TestGetFromLinkerValues(t *testing.T) {
	defer func(v, c string) { Version, Commit = v, c }(Version, Commit)

	Version, Commit = "v0.2.0", "0123456789abcdef"
	if got := Get(); got.Version != "0.2.0" || got.Commit != "0123456" || got.Modified {
		t.Errorf("Get() = %+v, want version 0.2.0, commit 0123456", got)
	}

	Version, Commit = "", "abc"
	if got := Get(); got.Version != "dev" {
		t.Errorf("empty version: Get().Version = %q, want dev", got.Version)
	}
}
