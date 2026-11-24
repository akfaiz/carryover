package settings

import (
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// selfSteamCard adds Carryover itself to Steam, so it can be opened from Steam
// Deck Game Mode and used with a controller.
type selfSteamCard struct {
	u     env.Env
	art   func() map[string][]byte // Carryover's library artwork by kind
	info  *services.SteamInfo      // nil while Steam is being detected
	user  string
	appID string // of Carryover's shortcut for user, "" if none
	busy  bool
}

// newSelfSteamCard is the card; art supplies the library artwork it sets.
func newSelfSteamCard(u env.Env, art func() map[string][]byte) *selfSteamCard {
	s := &selfSteamCard{u: u, art: art}
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
	})
	return s
}

func (s *selfSteamCard) view(c *ui.Context) {
	kit.Card(c, i18n.L("Steam"), func() {
		nav := s.u.Navigator()
		switch {
		case s.info == nil:
			kit.Muted(c, i18n.L("Detecting Steam…"))
			return
		case !s.info.Found:
			kit.Muted(c, i18n.L("Steam was not found on this device."))
			return
		}
		kit.Muted(c, i18n.L("Add Carryover to Steam as a non-Steam game to open it from Steam Deck Game Mode and use it with a controller. Its library artwork is set too."))
		if len(s.info.Users) > 1 {
			users := &kit.Choice{}
			for _, us := range s.info.Users {
				users.Add(us.ID, us.Name)
			}
			kit.Labeled(c, i18n.L("Steam account"), func() {
				kit.SelectOf(c, nav, s.u.Dialogs(), "steam-user", i18n.L("Steam account"), users, s.user, s.selectUser)
			})
		}
		if s.appID != "" {
			kit.Colored(c, "✓ "+i18n.L("In Steam on this device"), c.Theme().Success)
		}
		label := i18n.L("Add Carryover to Steam")
		if s.appID != "" {
			label = i18n.L("Update Steam shortcut")
		}
		ui.Row(c).Gap(8).Children(func() {
			if nav.Item(nav.Handle("self-steam-add"), ui.PrimaryButton(c.Key("self-steam-add"), label).Disabled(s.busy || s.user == ""), s.add).Clicked() {
				s.add()
			}
			if s.appID != "" {
				if nav.Item(nav.Handle("self-steam-rm"), ui.Button(c.Key("self-steam-rm"), i18n.L("Remove")).Disabled(s.busy), s.remove).Clicked() {
					s.remove()
				}
			}
		})
		if s.busy {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				kit.Muted(c, i18n.L("Updating Steam…"))
			})
		}
		if s.info.Running {
			kit.Colored(c, i18n.L("Steam is running — restart it after adding so it picks up the change."), c.Theme().Warning)
		}
	})
}

func (s *selfSteamCard) selectUser(id string) {
	if id == s.user {
		return
	}
	s.user, s.appID = id, ""
	kit.Async(func() (string, error) { return s.u.Steam().SelfAppID(id) }, func(appID string, _ error) {
		if s.user == id {
			s.appID = appID
		}
	})
}

func (s *selfSteamCard) add() {
	user := s.user
	s.busy = true
	kit.Async(func() (services.AddToSteamResult, error) {
		return s.u.Steam().AddSelf(user, s.art())
	}, func(res services.AddToSteamResult, err error) {
		s.busy = false
		if err != nil {
			s.u.ToastErr(err)
			return
		}
		s.appID = res.AppID
		if res.SteamRunning {
			s.u.Toast(i18n.L("Added to Steam. Restart Steam to see it."), kit.ToastOK)
		} else {
			s.u.Toast(i18n.L("Added to Steam"), kit.ToastOK)
		}
	})
}

func (s *selfSteamCard) remove() {
	user := s.user
	s.busy = true
	kit.Async(func() (int, error) { return s.u.Steam().RemoveSelf(user) }, func(n int, err error) {
		s.busy = false
		switch {
		case err != nil:
			s.u.ToastErr(err)
		case n > 0:
			s.appID = ""
			s.u.Toast(i18n.L("Removed from Steam"), kit.ToastOK)
		default:
			s.appID = ""
			s.u.Toast(i18n.L("No shortcut found"), kit.ToastOK)
		}
	})
}
