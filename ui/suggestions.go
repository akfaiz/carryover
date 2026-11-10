package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/playrelay/services"
)

var kindLabel = map[string]string{
	"native":     "Linux/macOS native",
	"proton":     "Proton prefix",
	"install":    "install folder",
	"steamcloud": "Steam Cloud folder",
}

// suggestionList shows the known save locations found for a game (from the
// Ludusavi manifest) and lets the user use or add one.
type suggestionList struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

// newSuggestionList builds the list for res. gameName is only used in the
// "nothing known" message.
func newSuggestionList(gameName string, res *services.SuggestResult, onUse, onAdd func(services.SaveSuggestion), onHide func()) *suggestionList {
	c := vbox()
	if res.Matched == "" {
		c.Add(muted("No known save locations for “" + gameName + "”. Try “Find on Steam” first, or set the path yourself."))
	} else {
		c.Add(container.NewBorder(nil, nil, nil, lowButton("Hide", onHide), muted("Known save locations for "+res.Matched)))
		if len(res.Suggestions) == 0 {
			c.Add(muted("None that apply to this device."))
		}
		for _, s := range res.Suggestions {
			c.Add(widget.NewSeparator())
			c.Add(suggestionRow(s, onUse, onAdd))
		}
		c.Add(hbox(small("From the"), widget.NewHyperlink("Ludusavi manifest", mustURL("https://github.com/mtkennerly/ludusavi-manifest")), small("(PCGamingWiki).")))
	}
	l := &suggestionList{content: c}
	l.ExtendBaseWidget(l)
	return l
}

// CreateRenderer implements fyne.Widget.
func (l *suggestionList) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(l.content) }

func suggestionRow(s services.SaveSuggestion, onUse, onAdd func(services.SaveSuggestion)) fyne.CanvasObject {
	text := s.Path
	var inc []string
	for _, x := range s.Include {
		if !strings.HasSuffix(x, "/**") {
			inc = append(inc, x)
		}
	}
	if len(inc) > 0 {
		text += " · " + strings.Join(inc, ", ")
	}
	path := mono(text)
	if s.Files > 0 {
		path.Importance = widget.SuccessImportance
	}
	badges := hbox()
	switch {
	case s.Files > 0:
		badges.Add(badge(fmt.Sprintf("found · %d files · %s · %s", s.Files, formatBytes(s.Bytes), timeAgo(s.Newest)), badgeOK))
	case s.Exists:
		badges.Add(badge("folder exists, no saves", badgeNone))
	default:
		badges.Add(badge("not on this device", badgeNone))
	}
	if l := kindLabel[s.Kind]; l != "" {
		badges.Add(badge(l, badgeNone))
	}
	if s.MSStore {
		badges.Add(badge("Microsoft Store version", badgeWarn))
	}
	actions := hbox(widget.NewButton("Use", func() { onUse(s) }), lowButton("＋ Add", func() { onAdd(s) }))
	row := vbox(path, container.NewBorder(nil, nil, nil, actions, container.NewHScroll(badges)))
	if s.Kind == "steamcloud" {
		row.Add(muted("Steam Cloud already syncs this for the Steam version; useful for non-Steam copies."))
	}
	return row
}
