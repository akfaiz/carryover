package components

import (
	"math"
	"time"

	"github.com/akfaiz/carryover/ui/i18n"
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
		return i18n.L("never")
	}
	s := time.Since(t).Seconds()
	switch {
	case s < 60:
		return i18n.L("just now")
	case s < 3600:
		return i18n.L("{{.N}}m ago", map[string]any{"N": int(math.Round(s / 60))})
	case s < 86400:
		return i18n.L("{{.N}}h ago", map[string]any{"N": int(math.Round(s / 3600))})
	}
	return t.Local().Format("2006-01-02")
}

func TimeAgo(s string) string { return timeAgoT(parseTime(s)) }

func FmtDate(t time.Time) string {
	if t.IsZero() || t.Year() <= 1 {
		return "—"
	}
	return t.Local().Format("2006-01-02 15:04")
}
