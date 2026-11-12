package services

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// romExt is the file extension (up to six letters or digits) of a ROM or disc image.
	romExt = regexp.MustCompile(`\.[A-Za-z0-9]{1,6}$`)
	// romTags are [..] and (..) groups: title IDs, regions, versions, dump info.
	romTags = regexp.MustCompile(`\s*(\[[^\]]*\]|\([^)]*\))`)
	// romVersion is a trailing version such as "v1.0.2" or "Update 3".
	romVersion = regexp.MustCompile(`(?i)[\s_.-]+(v\d[\d.]*|update\s*\d[\d.]*|rev\s*\d+)$`)
	// romTitleID is a Nintendo Switch title ID (16 hex digits starting 01).
	romTitleID = regexp.MustCompile(`(?i)\b01[0-9a-f]{14}\b`)
	// bracketID is an ID or serial in [brackets], e.g. [0100000000010000] or [SLUS-12345].
	bracketID = regexp.MustCompile(`\[([A-Za-z0-9][A-Za-z0-9-]{5,19})\]`)
)

// cleanRomName turns a ROM or disc file name into a game title: no extension,
// no [title id] / (region) / version tags, underscores as spaces.
func cleanRomName(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, `\`, "/"))
	name := filepath.Base(filepath.FromSlash(path))
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	if loc := romExt.FindStringIndex(name); loc != nil {
		name = name[:loc[0]]
	}
	name = romTags.ReplaceAllString(name, "")
	name = strings.ReplaceAll(name, "_", " ")
	for {
		trimmed := strings.TrimSpace(romVersion.ReplaceAllString(name, ""))
		if trimmed == name || trimmed == "" {
			break
		}
		name = trimmed
	}
	return strings.Join(strings.Fields(name), " ")
}

// romIDs lists the title IDs and serials a ROM file name carries: those in
// [brackets] (version tags like [v0] excluded), plus a bare Switch title ID.
func romIDs(rom string) []string {
	base := filepath.Base(filepath.FromSlash(strings.ReplaceAll(rom, `\`, "/")))
	var out []string
	add := func(id string) {
		for _, o := range out {
			if strings.EqualFold(o, id) {
				return
			}
		}
		out = append(out, id)
	}
	for _, m := range bracketID.FindAllStringSubmatch(base, -1) {
		if !isVersionTag(m[1]) {
			add(m[1])
		}
	}
	if id := romTitleID.FindString(base); id != "" {
		add(id)
	}
	return out
}

// isVersionTag reports whether s looks like "v0" or "v1.2.0".
func isVersionTag(s string) bool {
	rest, ok := strings.CutPrefix(strings.ToLower(s), "v")
	return ok && rest != "" && strings.Trim(rest, "0123456789.") == ""
}
