// Package version reports which build of Carryover is running.
//
// Release builds set the version (the git tag) and commit with
// scripts/stamp-version.sh, which writes zz_release.go in this package.
// Plain go builds can also use
//
//	-ldflags "-X github.com/akfaiz/carryover/internal/app/version.Version=v0.2.0"
//
// Other builds report "dev" and the commit Go records from the git checkout.
package version

import (
	"runtime/debug"
	"strings"
)

// Set by zz_release.go or -ldflags -X (see the package comment).
var (
	Version = "dev"
	Commit  = ""
)

// RepoURL is the project's home page.
const RepoURL = "https://github.com/akfaiz/carryover"

// Info describes the running build.
type Info struct {
	Version  string // "0.2.0", "0.2.0-rc1" or "dev"
	Commit   string // short commit hash, "" if unknown
	Modified bool   // built from a checkout with uncommitted changes
}

// Get returns the running build's version information.
func Get() Info {
	info := Info{Version: strings.TrimPrefix(strings.TrimSpace(Version), "v"), Commit: Commit}
	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit == "" {
		info.Commit, info.Modified = vcs()
	}
	if len(info.Commit) > 7 {
		info.Commit = info.Commit[:7]
	}
	return info
}

// IsRelease reports whether this is a tagged release build.
func (i Info) IsRelease() bool { return i.Version != "dev" }

// String returns e.g. "0.2.0 (abc1234)", "dev (abc1234, modified)" or "dev".
func (i Info) String() string {
	var extra []string
	if i.Commit != "" {
		extra = append(extra, i.Commit)
	}
	if i.Modified {
		extra = append(extra, "modified")
	}
	if len(extra) == 0 {
		return i.Version
	}
	return i.Version + " (" + strings.Join(extra, ", ") + ")"
}

// ReleaseURL is the GitHub release page of this version, or the list of
// releases for a development build.
func (i Info) ReleaseURL() string {
	if !i.IsRelease() {
		return RepoURL + "/releases"
	}
	return RepoURL + "/releases/tag/v" + i.Version
}

// vcs returns the commit recorded by the go command (empty when built
// without git information, e.g. in Docker without .git).
func vcs() (rev string, modified bool) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	return rev, modified
}
