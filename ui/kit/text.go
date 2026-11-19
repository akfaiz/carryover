package kit

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func FormatBytes(n int64) string {
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

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func Slug(s string) string {
	out := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if out == "" {
		return "saves"
	}
	return out
}

// splitLines splits on newlines and commas, trimming blanks.
func SplitLines(t string) []string {
	var out []string
	for _, x := range strings.FieldsFunc(t, func(r rune) bool { return r == '\n' || r == ',' }) {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func Heading(t string) *widget.Label {
	l := widget.NewLabelWithStyle(t, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameSubHeadingText
	return l
}

func Title(t string) *widget.Label {
	l := widget.NewLabelWithStyle(t, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameHeadingText
	return l
}

// Muted is small grey wrapping text.
func Muted(t string) *widget.Label {
	l := widget.NewLabel(t)
	l.Importance = widget.LowImportance
	l.Wrapping = fyne.TextWrapWord
	l.SizeName = theme.SizeNameCaptionText
	return l
}

func Colored(t string, imp widget.Importance) *widget.Label {
	l := widget.NewLabel(t)
	l.Importance = imp
	l.Wrapping = fyne.TextWrapWord
	return l
}

func Mono(t string) *widget.Label {
	l := widget.NewLabelWithStyle(t, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	l.Wrapping = fyne.TextWrapBreak
	return l
}

func MustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// WrapTitle is a heading that wraps instead of growing the layout.
func WrapTitle(t string) *widget.Label {
	l := Title(t)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// Small is a non-wrapping grey caption, for use inside rows.
func Small(t string) *widget.Label {
	l := widget.NewLabel(t)
	l.Importance = widget.LowImportance
	l.SizeName = theme.SizeNameCaptionText
	return l
}
