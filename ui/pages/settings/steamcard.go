package settings

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// selfSteamCard adds Carryover itself to Steam, so it can be opened from Steam
// Deck Game Mode and used with a controller.
type selfSteamCard struct {
	u     env.Env
	art   func() map[string][]byte // Carryover's library artwork by kind
	box   *fyne.Container
	info  *services.SteamInfo // nil while Steam is being detected
	user  string
	appID string // of Carryover's shortcut for user, "" if none
	busy  bool
}

// newSelfSteamCard is the card; art supplies the library artwork it sets.
func newSelfSteamCard(u env.Env, art func() map[string][]byte) fyne.CanvasObject {
	s := &selfSteamCard{u: u, art: art, box: container.NewStack()}
	s.render()
	type state struct {
		info        services.SteamInfo
		user, appID string
	}
	kit.Async(func() (state, error) {
		st := state{info: u.Steam().Info()}
		if len(st.info.Users) > 0 {
			st.user = st.info.Users[0].ID
			st.appID, _ = u.Steam().SelfAppID(st.user)
		}
		return st, nil
	}, func(st state, _ error) {
		s.info, s.user, s.appID = &st.info, st.user, st.appID
		s.render()
	})
	return kit.Card(lang.L("Steam"), s.box)
}

func (s *selfSteamCard) render() {
	s.box.Objects = []fyne.CanvasObject{s.content()}
	s.box.Refresh()
}

func (s *selfSteamCard) content() fyne.CanvasObject {
	switch {
	case s.info == nil:
		return kit.Muted(lang.L("Detecting Steam…"))
	case !s.info.Found:
		return kit.Muted(lang.L("Steam was not found on this device."))
	}
	c := kit.VBox(kit.Muted(lang.L("Add Carryover to Steam as a non-Steam game to open it from Steam Deck Game Mode and use it with a controller. Its library artwork is set too.")))
	if len(s.info.Users) > 1 {
		users := &kit.Choice{}
		for _, us := range s.info.Users {
			users.Add(us.ID, us.Name)
		}
		c.Add(kit.Field(lang.L("Steam account"), kit.Outlined(kit.SelectOf(users, s.user, s.selectUser))))
	}
	if s.appID != "" {
		c.Add(kit.HBox(widget.NewIcon(theme.NewSuccessThemedResource(theme.ConfirmIcon())),
			widget.NewLabel(lang.L("In Steam on this device"))))
	}
	label := lang.L("Add Carryover to Steam")
	if s.appID != "" {
		label = lang.L("Update Steam shortcut")
	}
	add := kit.Primary(label, s.add)
	kit.SetEnabled(add, !s.busy && s.user != "")
	row := kit.HBox(add)
	if s.appID != "" {
		rm := widget.NewButton(lang.L("Remove"), s.remove)
		kit.SetEnabled(rm, !s.busy)
		row.Add(rm)
	}
	c.Add(row)
	if s.busy {
		c.Add(kit.BusyLine(lang.L("Updating Steam…")))
	}
	if s.info.Running {
		c.Add(kit.Colored(lang.L("Steam is running — restart it after adding so it picks up the change."), widget.WarningImportance))
	}
	return c
}

func (s *selfSteamCard) selectUser(id string) {
	if id == s.user {
		return
	}
	s.user, s.appID = id, ""
	kit.Async(func() (string, error) { return s.u.Steam().SelfAppID(id) }, func(appID string, _ error) {
		if s.user == id {
			s.appID = appID
			s.render()
		}
	})
}

func (s *selfSteamCard) add() {
	user := s.user
	s.busy = true
	s.render()
	kit.Async(func() (services.AddToSteamResult, error) {
		return s.u.Steam().AddSelf(user, s.art())
	}, func(res services.AddToSteamResult, err error) {
		s.busy = false
		if err != nil {
			s.u.ToastErr(err)
		} else {
			s.appID = res.AppID
			if res.SteamRunning {
				s.u.Toast(lang.L("Added to Steam. Restart Steam to see it."), kit.ToastOK)
			} else {
				s.u.Toast(lang.L("Added to Steam"), kit.ToastOK)
			}
		}
		s.render()
	})
}

func (s *selfSteamCard) remove() {
	user := s.user
	s.busy = true
	s.render()
	kit.Async(func() (int, error) { return s.u.Steam().RemoveSelf(user) }, func(n int, err error) {
		s.busy = false
		switch {
		case err != nil:
			s.u.ToastErr(err)
		case n > 0:
			s.appID = ""
			s.u.Toast(lang.L("Removed from Steam"), kit.ToastOK)
		default:
			s.appID = ""
			s.u.Toast(lang.L("No shortcut found"), kit.ToastOK)
		}
		s.render()
	})
}
