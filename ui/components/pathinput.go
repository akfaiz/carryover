package components

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// PathInput is a path entry with token insertion, a folder browser and a live
// preview of the expanded path. A page keeps one per path and calls View each
// frame; Text is what is typed, SetText changes it from outside.
type PathInput struct {
	u           env.Env
	gameID      string
	tokens      []pathvar.Token
	Text        string
	Placeholder string
	onChange    func(string)

	last       string // Text as of the last frame, to notice typing
	preview    string // the expanded path, when it differs from Text
	previewErr string // or why it can't be expanded
	gen        int
	schedule   func()
}

// NewPathInput returns the entry for gameID's paths, showing value.
func NewPathInput(u env.Env, gameID string, tokens []pathvar.Token, value, placeholder string, onChange func(string)) *PathInput {
	p := &PathInput{u: u, gameID: gameID, tokens: tokens, Text: value, last: value, Placeholder: placeholder, onChange: onChange}
	p.schedule = kit.Debounce(250*time.Millisecond, p.updatePreview)
	p.updatePreview()
	return p
}

// Follow shows v when it differs from what the field showed at the last frame:
// the path was changed from outside (a suggestion was used). A path the user
// typed is left alone.
func (p *PathInput) Follow(v string) {
	if v != p.last && v != p.Text {
		p.Text = v
	}
}

// SetText replaces the path, as if typed.
func (p *PathInput) SetText(s string) { p.Text = s }

// View draws the entry; key tells it from the other entries on the page.
func (p *PathInput) View(c *ui.Context, key any) ui.Element {
	nav := p.u.Navigator()
	k := [2]any{"path", key}
	if p.Text != p.last {
		p.last = p.Text
		if p.onChange != nil {
			p.onChange(p.Text)
		}
		p.schedule()
	}
	return ui.Column(c).Gap(4).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			kit.Field(c, nav, k, &p.Text, p.Placeholder).Font("monospace").Grow(1)
			if len(p.tokens) > 0 {
				// Named, not "{ }": the tokens are what it inserts, which the
				// symbol alone didn't say.
				kit.Btn(c, nav, [2]any{"path-tokens", key}, kit.BtnSecondary, nil, i18n.L("Tokens"), p.showTokens)
			}
			kit.Btn(c, nav, [2]any{"path-browse", key}, kit.BtnSecondary, kit.IconFolderOpen, i18n.L("Browse…"), p.browse)
		})
		if p.preview != "" {
			kit.Small(c, "→ "+p.preview).Font("monospace").TextColor(c.Theme().TextMuted)
		}
		if p.previewErr != "" {
			kit.Danger(c, p.previewErr)
		}
	})
}

// showTokens lists the tokens in a dialog; picking one adds it to the path.
func (p *PathInput) showTokens() {
	var d *kit.Dialog
	d = p.u.Dialogs().Show(&kit.Dialog{Title: i18n.L("Tokens"), Width: 560, Body: func(c *ui.Context, nav *kit.Navigator) {
		for i, t := range p.tokens {
			desc := t.Value
			if desc == "" {
				desc = i18n.Text(t.Description, t.DescriptionMsg)
			}
			kit.Btn(c, nav, [3]any{"path-token", p.gameID, i}, kit.BtnSecondary, nil, t.Token+"   "+desc, func() { d.Close(); p.insert(t.Token) }).
				AlignSelf(ui.Stretch).Justify(ui.Start)
		}
	}})
}

// insert adds a token at the end of the path: MyGo's text input has no caret to
// ask for.
func (p *PathInput) insert(tok string) {
	p.Text += strings.Replace(tok, "<appid>", "", 1)
}

// browse picks a folder, starting at the one typed in (or the placeholder's,
// the folder used while none is typed).
func (p *PathInput) browse() {
	from := p.Text
	if strings.TrimSpace(from) == "" {
		from = p.Placeholder
	}
	BrowseFrom(p.u, p.gameID, from, func(start string) {
		kit.PickDir(p.u.Window(), i18n.L("Choose a folder"), start, func(dir string) {
			p.Text = p.u.Lib().ContractPath(filepath.FromSlash(dir))
		})
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
	v := p.Text
	p.gen++
	gen := p.gen
	if v == "" {
		p.preview, p.previewErr = "", ""
		return
	}
	kit.Async(func() (string, error) { return p.u.Lib().PreviewPath(p.gameID, v) }, func(out string, err error) {
		if gen != p.gen {
			return
		}
		p.preview, p.previewErr = "", ""
		switch {
		case err != nil:
			p.previewErr = i18n.Err(err)
		case out != "" && out != v:
			p.preview = out
		}
	})
}
