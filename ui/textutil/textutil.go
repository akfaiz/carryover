// Package textutil holds the GUI's plain string helpers. It imports no GUI
// toolkit, so ViewModels can use it.
package textutil

import (
	"regexp"
	"strings"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug turns a label into a save location key: lower case, dashes for
// everything else, "saves" when nothing is left.
func Slug(s string) string {
	out := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if out == "" {
		return "saves"
	}
	return out
}

// SplitLines splits on newlines and commas, trimming blanks.
func SplitLines(t string) []string {
	var out []string
	for _, x := range strings.FieldsFunc(t, func(r rune) bool { return r == '\n' || r == ',' }) {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// LabelOf is a save location's label, or its key when it has none.
func LabelOf(label, key string) string {
	if label != "" {
		return label
	}
	return key
}
