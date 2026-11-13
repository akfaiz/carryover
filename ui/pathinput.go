package ui

import (
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/internal/ui/kit"
)

// pathInput is a path entry with token insertion, a folder browser and a
// live preview of the expanded path.
type pathInput struct {
	kit.Composite
	u       *UI
	entry   *widget.Entry
	preview *widget.Label
	gameID  string
	gen     int
}

func newPathInput(u *UI, gameID string, tokens []pathvar.Token, value, placeholder string, onChange func(string)) *pathInput {
	p := &pathInput{u: u, gameID: gameID, entry: widget.NewEntry(), preview: kit.Mono("")}
	p.entry.TextStyle.Monospace = true
	p.entry.SetPlaceHolder(placeholder)
	p.entry.SetText(value)
	p.preview.SizeName = theme.SizeNameCaptionText
	p.preview.Hide()
	schedule := kit.Debounce(250*time.Millisecond, p.updatePreview)
	p.entry.OnChanged = func(s string) {
		if onChange != nil {
			onChange(s)
		}
		schedule()
	}
	var right []fyne.CanvasObject
	if len(tokens) > 0 {
		var tokBtn *widget.Button
		tokBtn = widget.NewButton("{ }", func() {
			items := make([]*fyne.MenuItem, 0, len(tokens))
			for _, t := range tokens {
				desc := t.Value
				if desc == "" {
					desc = t.Description
				}
				items = append(items, fyne.NewMenuItem(t.Token+"   "+desc, func() { p.insert(t.Token) }))
			}
			c := fyne.CurrentApp().Driver().CanvasForObject(tokBtn)
			pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(tokBtn)
			widget.ShowPopUpMenuAtPosition(fyne.NewMenu("", items...), c, pos.AddXY(0, tokBtn.Size().Height))
		})
		right = append(right, tokBtn)
	}
	right = append(right, widget.NewButton(lang.L("Browse…"), p.browse))
	p.Init(p, kit.VBox(kit.RowFill(p.entry, right...), p.preview))
	p.updatePreview()
	return p
}

func (p *pathInput) insert(tok string) {
	tok = strings.Replace(tok, "<appid>", "", 1)
	runes := []rune(p.entry.Text)
	col := min(max(p.entry.CursorColumn, 0), len(runes))
	text := string(runes[:col]) + tok + string(runes[col:])
	pos := col + len([]rune(tok))
	if strings.HasSuffix(tok, ":}") { // e.g. {proton:} — put the cursor before "}"
		pos--
	}
	p.entry.SetText(text)
	p.entry.CursorColumn = pos
	p.entry.Refresh()
	if c := fyne.CurrentApp().Driver().CanvasForObject(p.entry); c != nil {
		c.Focus(p.entry)
	}
}

func (p *pathInput) browse() {
	d := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil || dir == nil {
			return
		}
		p.entry.SetText(p.u.lib.ContractPath(filepath.FromSlash(dir.Path())))
	}, p.u.win)
	d.Resize(kit.DialogSize(p.u.win))
	d.Show()
}

func (p *pathInput) updatePreview() {
	v := p.entry.Text
	p.gen++
	gen := p.gen
	if v == "" {
		p.preview.Hide()
		return
	}
	kit.Async(func() (string, error) { return p.u.lib.PreviewPath(p.gameID, v) }, func(out string, err error) {
		if gen != p.gen {
			return
		}
		switch {
		case err != nil:
			p.preview.Importance = widget.DangerImportance
			p.preview.SetText(err.Error())
			p.preview.Show()
		case out != "" && out != v:
			p.preview.Importance = widget.LowImportance
			p.preview.SetText("→ " + out)
			p.preview.Show()
		default:
			p.preview.Hide()
		}
	})
}

// dialogSize is a comfortable size for file and picker dialogs.
