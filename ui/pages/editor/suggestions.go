package editor

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/kit"
)

// kindLabel names a kind of save-location suggestion.
func kindLabel(kind string) string {
	switch kind {
	case "native":
		return i18n.L("Linux/macOS native")
	case "proton":
		return i18n.L("Proton prefix")
	case "install":
		return i18n.L("install folder")
	case "steamcloud":
		return i18n.L("Steam Cloud folder")
	case "prefix":
		return i18n.L("Wine prefix")
	}
	return ""
}

// suggestionList shows the known save locations found for a game (from the
// Ludusavi manifest) and lets the user use or add one.
type suggestionList struct{ kit.Composite }

// newSuggestionList builds the list for res. gameName is only used in the
// "nothing known" message.
func newSuggestionList(gameName string, res *services.SuggestResult, onUse, onAdd func(services.SaveSuggestion), onHide func()) *suggestionList {
	c := kit.VBox()
	if res.Matched == "" {
		c.Add(kit.Muted(i18n.L("No known save locations for “{{.Game}}”. Try “Find on Steam” first, or set the path yourself.", map[string]any{"Game": gameName})))
	} else {
		c.Add(container.NewBorder(nil, nil, nil, kit.LowButton(i18n.L("Hide"), onHide), kit.Muted(i18n.L("Known save locations for {{.Game}}", map[string]any{"Game": res.Matched}))))
		if len(res.Suggestions) == 0 {
			c.Add(kit.Muted(i18n.L("None that apply to this device.")))
		}
		for _, s := range res.Suggestions {
			c.Add(widget.NewSeparator())
			c.Add(suggestionRow(s, onUse, onAdd))
		}
		c.Add(kit.HBox(kit.Small(i18n.L("From the")), widget.NewHyperlink(i18n.L("Ludusavi manifest"), kit.MustURL("https://github.com/mtkennerly/ludusavi-manifest")), kit.Small(i18n.L("(PCGamingWiki)."))))
	}
	l := &suggestionList{}
	l.Init(l, c)
	return l
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
	path := kit.Mono(text)
	if s.Files > 0 {
		path.Importance = widget.SuccessImportance
	}
	badges := kit.HBox()
	switch {
	case s.Files > 0:
		badges.Add(kit.Badge(i18n.N("found · {{.Count}} file · {{.Size}} · {{.Age}}", s.Files, map[string]any{"Count": s.Files, "Size": kit.FormatBytes(s.Bytes), "Age": components.TimeAgo(s.Newest)}), kit.BadgeOK))
	case s.Exists:
		badges.Add(kit.Badge(i18n.L("Folder exists, no saves"), kit.BadgeNone))
	default:
		badges.Add(kit.Badge(i18n.L("Not on this device"), kit.BadgeNone))
	}
	if l := kindLabel(s.Kind); l != "" {
		if s.Prefix != "" {
			l = s.Prefix // which launcher's prefix, e.g. "Heroic: Hades"
		}
		badges.Add(kit.Badge(l, kit.BadgeNone))
	}
	if s.MSStore {
		badges.Add(kit.Badge(i18n.L("Microsoft Store version"), kit.BadgeWarn))
	}
	actions := kit.HBox(widget.NewButton(i18n.L("Use"), func() { onUse(s) }), kit.LowButton(i18n.L("＋ Add"), func() { onAdd(s) }))
	row := kit.VBox(path, container.NewBorder(nil, nil, nil, actions, container.NewHScroll(badges)))
	if s.Kind == "steamcloud" {
		row.Add(kit.Muted(i18n.L("Steam Cloud already syncs this for the Steam version; useful for non-Steam copies.")))
	}
	return row
}
