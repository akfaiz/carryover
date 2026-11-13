package ui

import (
	"math"
	"regexp"
	"strings"
	"time"

	"fyne.io/fyne/v2/lang"
)

// parseTime parses the RFC3339 strings the services return ("" → zero).
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func timeAgoT(t time.Time) string {
	if t.IsZero() || t.Year() <= 1 {
		return lang.L("never")
	}
	s := time.Since(t).Seconds()
	switch {
	case s < 60:
		return lang.L("just now")
	case s < 3600:
		return lang.L("{{.N}} min ago", map[string]any{"N": int(math.Round(s / 60))})
	case s < 86400:
		return lang.L("{{.N}} h ago", map[string]any{"N": int(math.Round(s / 3600))})
	}
	return t.Local().Format("2006-01-02")
}

func timeAgo(s string) string { return timeAgoT(parseTime(s)) }

func fmtDate(t time.Time) string {
	if t.IsZero() || t.Year() <= 1 {
		return "—"
	}
	return t.Local().Format("2006-01-02 15:04")
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	out := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if out == "" {
		return "saves"
	}
	return out
}

// splitLines splits on newlines and commas, trimming blanks.
func splitLines(t string) []string {
	var out []string
	for _, x := range strings.FieldsFunc(t, func(r rune) bool { return r == '\n' || r == ',' }) {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}
