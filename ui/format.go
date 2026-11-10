package ui

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"fyne.io/fyne/v2/lang"
)

func formatBytes(n int64) string {
	if n <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", f, units[i])
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

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

func initial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}

func pct(bytes, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Min(1, float64(bytes)/float64(total))
}
