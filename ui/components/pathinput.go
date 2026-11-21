package components

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
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// PathInput is a path entry with token insertion, a folder browser and a
// live preview of the expanded path.
type PathInput struct {
	kit.Composite
	u        env.Env
	entry    *widget.Entry
	preview  *kit.PathLabel // the expanded path, on one line
	previewE *widget.Label  // or why it can't be expanded
	gameID   string
	gen      int
}

func NewPathInput(u env.Env, gameID string, tokens []pathvar.Token, value, placeholder string, onChange func(string)) *PathInput {
	p := &PathInput{u: u, gameID: gameID, entry: widget.NewEntry(), preview: kit.NewPathLabel("").Caption("→ "),
		previewE: kit.Colored("", widget.DangerImportance)}
	p.entry.TextStyle.Monospace = true
	p.entry.SetPlaceHolder(placeholder)
	p.entry.SetText(value)
	p.preview.Hide()
	p.previewE.Hide()
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
		// Named, not "{ }": the tokens are what it inserts, which the
		// symbol alone didn't say.
		tokBtn = widget.NewButtonWithIcon(lang.L("Tokens"), theme.MenuDropDownIcon(), func() {
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
	p.Init(p, kit.VBox(kit.RowFill(p.entry, right...), p.preview, p.previewE))
	p.updatePreview()
	return p
}

func (p *PathInput) insert(tok string) {
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

// browse picks a folder, starting at the one typed in (or the placeholder's,
// the folder used while none is typed).
func (p *PathInput) browse() {
	from := p.entry.Text
	if strings.TrimSpace(from) == "" {
		from = p.entry.PlaceHolder
	}
	BrowseFrom(p.u, p.gameID, from, func(start string) {
		d := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
			if err != nil || dir == nil {
				return
			}
			p.entry.SetText(p.u.Lib().ContractPath(filepath.FromSlash(dir.Path())))
		}, p.u.Window())
		kit.StartIn(d, start)
		kit.ShowFilePicker(p.u.Window(), d)
	})
}

// BrowseFrom finds, off the UI goroutine, the folder a file or folder dialog
// for raw should open at: raw (a path that may contain tokens, expanded for
// gameID) if it is a folder, else the nearest folder above it that exists.
// show then opens the dialog there ("" for the dialog's default: nothing set,
// a token that can't be expanded, or nothing of the path exists).
func BrowseFrom(u env.Env, gameID, raw string, show func(dir string)) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		show("")
		return
	}
	kit.Async(func() (string, error) {
		p, err := u.Lib().PreviewPath(gameID, raw)
		if err != nil {
			return "", nil //nolint:nilerr // unexpandable: the dialog's default
		}
		return kit.ExistingDir(p), nil
	}, func(dir string, _ error) { show(dir) })
}

func (p *PathInput) updatePreview() {
	v := p.entry.Text
	p.gen++
	gen := p.gen
	if v == "" {
		p.preview.Hide()
		p.previewE.Hide()
		return
	}
	kit.Async(func() (string, error) { return p.u.Lib().PreviewPath(p.gameID, v) }, func(out string, err error) {
		if gen != p.gen {
			return
		}
		p.preview.Hide()
		p.previewE.Hide()
		switch {
		case err != nil:
			p.previewE.SetText(err.Error())
			p.previewE.Show()
		case out != "" && out != v:
			p.preview.SetPath(out)
			p.preview.Show()
		}
	})
}

// dialogSize is a comfortable size for file and picker dialogs.
