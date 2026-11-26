// Package update finds newer releases of Carryover on GitHub, downloads and
// verifies them, and replaces the installed program with them. It knows how
// the release files are named and laid out (.github/workflows/release.yml);
// what to tell the user, and when, is services.UpdateService's business.
package update

import (
	"cmp"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version is a parsed release version such as 1.2.0-beta.2.
type Version struct {
	Major, Minor, Patch int
	// Pre are the dot-separated parts of the pre-release label ("beta", "2");
	// empty for a full release.
	Pre []string
}

var (
	versionRe = regexp.MustCompile(`^v?(\d{1,9})\.(\d{1,9})\.(\d{1,9})(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z.-]+)?$`)
	// "rc1" is how the first release candidates were tagged: it orders as "rc.1".
	labelRe = regexp.MustCompile(`^([A-Za-z]+)(\d+)$`)
)

// Parse parses a version or release tag ("v1.2.0", "1.2.0-alpha.1"). A
// development build ("dev") is not a version.
func Parse(s string) (Version, error) {
	m := versionRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Version{}, fmt.Errorf("not a release version: %q", s)
	}
	var v Version
	v.Major, _ = strconv.Atoi(m[1])
	v.Minor, _ = strconv.Atoi(m[2])
	v.Patch, _ = strconv.Atoi(m[3])
	if m[4] != "" {
		for part := range strings.SplitSeq(m[4], ".") {
			if l := labelRe.FindStringSubmatch(part); l != nil {
				v.Pre = append(v.Pre, l[1], l[2])
				continue
			}
			v.Pre = append(v.Pre, part)
		}
	}
	return v, nil
}

// Prerelease reports whether v is an alpha, beta or release candidate.
func (v Version) Prerelease() bool { return len(v.Pre) > 0 }

// Compare returns -1, 0 or 1 as a is older than, the same as or newer than b,
// by the rules of semantic versioning: a release is newer than its
// pre-releases, and alpha < beta < rc, each by number.
func Compare(a, b Version) int {
	if c := cmp.Or(cmp.Compare(a.Major, b.Major), cmp.Compare(a.Minor, b.Minor), cmp.Compare(a.Patch, b.Patch)); c != 0 {
		return c
	}
	switch {
	case !a.Prerelease() && !b.Prerelease():
		return 0
	case !a.Prerelease():
		return 1
	case !b.Prerelease():
		return -1
	}
	for i := 0; i < len(a.Pre) && i < len(b.Pre); i++ {
		if c := comparePart(a.Pre[i], b.Pre[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a.Pre), len(b.Pre))
}

// comparePart compares one part of a pre-release label: numbers by value,
// words alphabetically, and a number before a word.
func comparePart(a, b string) int {
	x, errA := strconv.Atoi(a)
	y, errB := strconv.Atoi(b)
	switch {
	case errA == nil && errB == nil:
		return cmp.Compare(x, y)
	case errA == nil:
		return -1
	case errB == nil:
		return 1
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// Newer reports whether version latest is newer than the running version.
// It is false when either isn't a release version, so a development build
// never updates and switching back to the stable channel never downgrades.
func Newer(latest, current string) bool {
	l, errL := Parse(latest)
	c, errC := Parse(current)
	return errL == nil && errC == nil && Compare(l, c) > 0
}
