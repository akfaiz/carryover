package prompts

import (
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// emuState is the "which emulator?" prompt, used by the launcher's Play and by
// a Steam shortcut: a square tile per way to play the game, the default
// highlighted, and (for a Steam shortcut) a countdown after which the default
// starts. Clicking any choice answers at once; "Stop countdown" keeps the
// dialog open until a choice is made. Any press of the gamepad stops the
// countdown too: whoever is moving between the choices is choosing.
type emuState struct {
	q      *Queue
	p      services.Prompt
	answer func(string)

	remaining int
	paused    bool
	stop      chan struct{}
	once      sync.Once
}

func newEmuState(q *Queue, p services.Prompt, answer func(string)) *emuState {
	s := &emuState{q: q, p: p, answer: answer, remaining: p.Seconds, stop: make(chan struct{})}
	q.e.Navigator().OnInput = s.pause
	if p.Seconds > 0 {
		go s.tick()
	}
	return s
}

// stopCountdown ends the ticker; cancelled says the prompt is going away.
func (s *emuState) stopCountdown(bool) { s.once.Do(func() { close(s.stop) }) }

func (s *emuState) pause() {
	if !s.paused {
		s.paused = true
		kit.Redraw()
	}
}

// finish answers once, whoever asks first (a tile, the countdown, B).
func (s *emuState) finish(a string) {
	s.stopCountdown(false)
	s.answer(a)
}

func (s *emuState) tick() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			kit.OnUI(func() {
				select {
				case <-s.stop:
					return
				default:
				}
				if len(s.q.queue) == 0 || s.q.queue[0].ID != s.p.ID {
					s.stopCountdown(false) // the prompt was withdrawn
					return
				}
				if s.paused {
					return
				}
				s.remaining--
				if s.remaining <= 0 {
					s.finish(s.p.Default)
				}
			})
		}
	}
}

func choiceName(c services.ProfileChoice) string {
	if c.ID == services.NativeProfile {
		return i18n.L("Without an emulator")
	}
	return c.Name
}

func (s *emuState) body(c *ui.Context, nav *kit.Navigator) {
	p := s.p
	hint := i18n.L("Choose how to play. The highlighted one is the one you played last.")
	if p.Seconds > 0 {
		hint = i18n.L("Choose how to play. The highlighted one starts by itself when the countdown ends.")
	}
	kit.Muted(c, hint)

	var defName string
	for _, ch := range p.Choices {
		if ch.ID == p.Default {
			defName = choiceName(ch)
		}
	}
	width := float32(kit.ChoiceTileSize)
	if d := s.q.current; d != nil {
		width = d.ContentWidth(c)
	}
	kit.ChoiceGrid(c, width, len(p.Choices), func(i int) {
		ch := p.Choices[i]
		icon := kit.IconComputer
		if ch.ID == services.NativeProfile {
			icon = kit.IconPlay // the game itself, no emulator
		}
		// The default tile is where the dialog starts (and is safe: it is what
		// would start by itself).
		var opts []kit.ItemOption
		if ch.ID == p.Default {
			opts = append(opts, kit.FocusSafe)
		}
		kit.ChoiceTile(c, nav, ch.ID, choiceName(ch), icon, ch.ID == p.Default, func() { s.finish(ch.ID) }, opts...)
	})
	if p.Seconds > 0 && !s.paused && defName != "" {
		kit.Muted(c, i18n.N("Starting with {{.Emulator}} in {{.Count}} second", s.remaining,
			map[string]any{"Emulator": defName, "Count": s.remaining}))
	}
	kit.Footer(c, func() {
		if p.Seconds > 0 && !s.paused {
			kit.Btn(c, nav, "emu-pause", kit.BtnSecondary, nil, i18n.L("Stop countdown"), s.pause)
		}
		kit.Btn(c, nav, "emu-cancel", kit.BtnSecondary, nil, i18n.L("Cancel"), func() { s.finish("cancel") })
	})
}
