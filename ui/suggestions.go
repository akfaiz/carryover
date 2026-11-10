package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
)

// kindLabel names a kind of save-location suggestion.
func kindLabel(kind string) string {
	switch kind {
	case "native":
		return lang.L("Linux/macOS native")
	case "proton":
		return lang.L("Proton prefix")
	case "install":
		return lang.L("install folder")
	case "steamcloud":
		return lang.L("Steam Cloud folder")
	}
	return ""
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
		c.Add(muted(lang.L("No known save locations for “{{.Game}}”. Try “Find on Steam” first, or set the path yourself.", map[string]any{"Game": gameName})))
	} else {
		c.Add(container.NewBorder(nil, nil, nil, lowButton(lang.L("Hide"), onHide), muted(lang.L("Known save locations for {{.Game}}", map[string]any{"Game": res.Matched}))))
		if len(res.Suggestions) == 0 {
			c.Add(muted(lang.L("None that apply to this device.")))
		}
		for _, s := range res.Suggestions {
			c.Add(widget.NewSeparator())
			c.Add(suggestionRow(s, onUse, onAdd))
		}
		c.Add(hbox(small(lang.L("From the")), widget.NewHyperlink(lang.L("Ludusavi manifest"), mustURL("https://github.com/mtkennerly/ludusavi-manifest")), small(lang.L("(PCGamingWiki)."))))
	}
	l := &suggestionList{content: c}
	l.ExtendBaseWidget(l)
	return l
}

// CreateRenderer implements fyne.Widget.
func (l *suggestionList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(l.content)
}

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
		badges.Add(badge(lang.N("found · {{.Count}} file · {{.Size}} · {{.Age}}", s.Files, map[string]any{"Count": s.Files, "Size": formatBytes(s.Bytes), "Age": timeAgo(s.Newest)}), badgeOK))
	case s.Exists:
		badges.Add(badge(lang.L("folder exists, no saves"), badgeNone))
	default:
		badges.Add(badge(lang.L("not on this device"), badgeNone))
	}
	if l := kindLabel(s.Kind); l != "" {
		badges.Add(badge(l, badgeNone))
	}
	if s.MSStore {
		badges.Add(badge(lang.L("Microsoft Store version"), badgeWarn))
	}
	actions := hbox(widget.NewButton(lang.L("Use"), func() { onUse(s) }), lowButton(lang.L("＋ Add"), func() { onAdd(s) }))
	row := vbox(path, container.NewBorder(nil, nil, nil, actions, container.NewHScroll(badges)))
	if s.Kind == "steamcloud" {
		row.Add(muted(lang.L("Steam Cloud already syncs this for the Steam version; useful for non-Steam copies.")))
	}
	return row
}
