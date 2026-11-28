package mobile

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
)

// Kinds of a game's save locations whose folder differs per device (see
// services.EmuSuggestion.Kind): the slot's key is the kind, on every device.
const (
	saveKindAccount = "account"
	saveKindDevice  = "device"
)

// SuggestSaves looks up where the emulator of profile key keeps this game's
// saves, in the per-game manifests of ludusavi-emulator-manifests
// (downloaded on first use), under the emulator's data folder on this device
// (which may be a folder granted through the Storage Access Framework). rom is
// the game file's name or path, which gives the most precise match (a Switch
// title ID in its name); "" uses the one the profile has. It returns a
// services.EmuSuggestResult as JSON; take a suggestion with UseSaveSuggestion.
func SuggestSaves(key, rom string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	res, _, err := suggest(s, key, rom)
	return encode(res, err)
}

// suggest is SuggestSaves with the game, for the callers that apply the
// result: the suggestions that make no sense for this game are left out.
func suggest(s *state, key, rom string) (services.EmuSuggestResult, library.Game, error) {
	gameID, emuID := core.ParseKey(key)
	if emuID == "" {
		return services.EmuSuggestResult{}, library.Game{}, msg.SuggestOnlyEmulator.Err()
	}
	g, err := s.core.Library.Get(gameID)
	if err != nil {
		return services.EmuSuggestResult{}, g, err
	}
	if strings.TrimSpace(rom) == "" {
		rom = g.Devices[s.core.DeviceID()].Emulators[emuID].Rom
	}
	rom = romFileName(rom)
	if emu, err := s.core.Library.Emulator(emuID); err == nil && emu.Preset == "switch" {
		rom = withTitleID(rom, otherDevicesTitleID(g, s.core.DeviceID()))
	}
	res, err := s.emus.SuggestSaves(emuID, services.EmuSuggestInput{GameName: g.Name, Rom: rom})
	if err != nil {
		return res, g, err
	}
	res.Suggestions = slices.DeleteFunc(res.Suggestions, func(sg services.EmuSuggestion) bool { return hideSuggestion(g, emuID, sg) })
	if res.Suggestions == nil {
		res.Suggestions = []services.EmuSuggestion{}
	}
	return res, g, nil
}

// romFileName is the file name in rom, a file path or a content:// URI (whose
// document ID, URL-escaped, ends in the file's path), so that the [title id]
// in it can be found. Anything else is returned as it is.
func romFileName(rom string) string {
	rom = strings.TrimSpace(rom)
	if !strings.HasPrefix(rom, "content://") {
		return rom
	}
	last := rom[strings.LastIndex(rom, "/")+1:]
	if name, err := url.PathUnescape(last); err == nil {
		last = name
	}
	return last[strings.LastIndexAny(last, "/:")+1:]
}

var (
	// switchSave matches a yuzu-family save folder: .../<profile>/<title id>.
	switchSave = regexp.MustCompile(`(?i)nand[/\\]user[/\\]save[/\\]0000000000000000[/\\][0-9a-f]{32}[/\\]([0-9a-f]{16})[/\\]?$`)
	// switchTitleID is a Switch title ID in a file name.
	switchTitleID = regexp.MustCompile(`(?i)\b01[0-9a-f]{14}\b`)
)

// otherDevicesTitleID is the Switch title ID in the save folders another
// device uses for g with any emulator ("" if none): the game file here may have
// no title ID in its name, but the save folders of the same game elsewhere do.
func otherDevicesTitleID(g library.Game, thisDevice string) string {
	devs := make([]string, 0, len(g.Devices))
	for id := range g.Devices {
		if id != thisDevice {
			devs = append(devs, id)
		}
	}
	sort.Strings(devs)
	for _, id := range devs {
		for _, ec := range g.Devices[id].Emulators {
			keys := make([]string, 0, len(ec.Paths))
			for k := range ec.Paths {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if m := switchSave.FindStringSubmatch(ec.Paths[k]); m != nil {
					return strings.ToUpper(m[1])
				}
			}
		}
	}
	return ""
}

// withTitleID adds a title ID to a game file name for the lookup, unless the
// name has one already (the game file knows best).
func withTitleID(rom, titleID string) string {
	if titleID == "" || switchTitleID.MatchString(rom) {
		return rom
	}
	return strings.TrimSpace(rom + " [" + titleID + "]")
}

// hideSuggestion reports whether a suggestion is left out of the list: a
// device save that isn't on this device, unless the game already has a device
// save slot (from another device, whose path this device then needs).
// yuzu-family emulators always name the device save folder, but most games
// never make one, so offering it would only add a location with nothing in it.
func hideSuggestion(g library.Game, emuID string, sg services.EmuSuggestion) bool {
	if sg.Kind != saveKindDevice || sg.Exists {
		return false
	}
	ge, _ := g.Emulator(emuID)
	return !slices.ContainsFunc(ge.Slots, func(sl library.SaveSlot) bool { return sl.Key == saveKindDevice })
}

// UseSaveSuggestion makes a suggestion (an services.EmuSuggestion as JSON,
// as SuggestSaves returned it) this device's save folder of profile key: a
// suggestion of a kind (account or device save) goes into the slot of that
// name, which every device shares, and the others into the first slot with no
// folder here, or into a new one. It returns the saved services.GameView.
func UseSaveSuggestion(key, suggestionJSON string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	var sg services.EmuSuggestion
	if err := json.Unmarshal([]byte(suggestionJSON), &sg); err != nil {
		return "", err
	}
	gameID, emuID := core.ParseKey(key)
	if emuID == "" {
		return "", msg.SuggestOnlyEmulator.Err()
	}
	g, err := s.core.Library.Get(gameID)
	if err != nil {
		return "", err
	}
	v, err := s.library.Get(gameID)
	if err != nil {
		return "", err
	}
	in := inputOf(v)
	if err := useSuggestion(&in, g, s.core.DeviceID(), emuID, sg); err != nil {
		return "", err
	}
	return encode(s.library.Save(in))
}

// AutofillSaveResult is what AutofillSaves did.
type AutofillSaveResult struct {
	// Applied is how many save folders were filled in.
	Applied int `json:"applied"`
}

// AutofillSaves fills in the save folders of profile key that have none on
// this device yet, with the locations SuggestSaves finds on this device (and,
// for a kind of save the game already has a slot for, the one candidate even
// before it exists). It changes nothing once the profile has a folder, and
// does nothing for an emulator that keeps every game's saves together. It
// returns an AutofillSaveResult as JSON. A lookup that fails (no data folder
// yet, no network for the manifest) is an error to tell the user only when
// they asked: the caller may ignore it.
func AutofillSaves(key, rom string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	res, g, err := suggest(s, key, rom)
	if err != nil {
		return "", err
	}
	_, emuID := core.ParseKey(key)
	v, err := s.library.Get(g.ID)
	if err != nil {
		return "", err
	}
	in := inputOf(v)
	n := autofill(&in, g, s.core.DeviceID(), emuID, res)
	if n == 0 {
		return encode(AutofillSaveResult{}, nil)
	}
	if _, err := s.library.Save(in); err != nil {
		return "", err
	}
	return encode(AutofillSaveResult{Applied: n}, nil)
}

// autofill applies the suggestions of res that can be trusted without asking
// to the emulator profile emuID of in, and returns how many it applied.
func autofill(in *services.GameInput, g library.Game, thisDevice, emuID string, res services.EmuSuggestResult) int {
	idx := slices.IndexFunc(in.Emulators, func(e services.GameEmulatorInput) bool { return e.EmulatorID == emuID })
	if idx < 0 || !res.PerGame {
		return 0
	}
	for _, p := range in.Emulators[idx].Paths {
		if strings.TrimSpace(p) != "" {
			return 0 // the user, or another suggestion, has already chosen
		}
	}
	candidates := map[string]int{} // by kind
	for _, sg := range res.Suggestions {
		if sg.Kind != "" {
			candidates[sg.Kind]++
		}
	}
	applied := 0
	for _, sg := range res.Suggestions {
		if sg.Kind != "" {
			hasSlot := slices.ContainsFunc(in.Emulators[idx].Slots, func(sl library.SaveSlot) bool { return sl.Key == sg.Kind })
			// With several profiles the user chooses; a slot the game already has gets this device's folder
			// even before the emulator made it.
			if candidates[sg.Kind] != 1 || (!sg.Exists && !hasSlot) {
				continue
			}
		} else if !sg.Exists {
			continue
		}
		if useSuggestion(in, g, thisDevice, emuID, sg) == nil {
			applied++
		}
	}
	return applied
}

var (
	errSharedSaves = msg.SuggestSharedSaves.Err()
	errNoFolder    = msg.SuggestNoFolder.Err()
)

// useSuggestion puts sg into the emulator profile emuID of in (see
// UseSaveSuggestion); g is the game as stored, which knows what the other
// devices use.
func useSuggestion(in *services.GameInput, g library.Game, thisDevice, emuID string, sg services.EmuSuggestion) error {
	idx := slices.IndexFunc(in.Emulators, func(e services.GameEmulatorInput) bool { return e.EmulatorID == emuID })
	if idx < 0 {
		return msg.EditNoEmulator.Err("Emulator", emuID)
	}
	if strings.TrimSpace(sg.Path) == "" {
		return errNoFolder
	}
	ge := &in.Emulators[idx]
	ge.Slots = slices.Clone(ge.Slots)
	if ge.Paths == nil {
		ge.Paths = map[string]string{}
	}
	if sg.Kind != "" {
		return useKind(ge, g, thisDevice, emuID, sg)
	}
	if ge.ShareWith != "" {
		return errSharedSaves
	}
	include := slices.Clone(sg.Include)
	for i, sl := range ge.Slots {
		if strings.TrimSpace(ge.Paths[sl.Key]) == "" {
			ge.Slots[i].Include = include
			ge.Paths[sl.Key] = sg.Path
			return nil
		}
	}
	key := uniqueSlotKey(ge.Slots, len(ge.Slots)+1)
	ge.Slots = append(ge.Slots, library.SaveSlot{Key: key, Label: fmt.Sprintf("Saves %d", len(ge.Slots)+1), Include: include})
	ge.Paths[key] = sg.Path
	return nil
}

// useKind puts a suggestion of a kind into the slot called that, which every
// device shares: this device only sets its own folder. Without one, a default
// slot that nothing has used becomes it (the key names the cloud folder, so
// only a slot no device has a folder in may be renamed), or a new slot is added.
func useKind(ge *services.GameEmulatorInput, g library.Game, thisDevice, emuID string, sg services.EmuSuggestion) error {
	if slices.ContainsFunc(ge.Slots, func(sl library.SaveSlot) bool { return sl.Key == sg.Kind }) {
		ge.Paths[sg.Kind] = sg.Path
		return nil
	}
	if ge.ShareWith != "" {
		return errSharedSaves
	}
	slot := library.SaveSlot{Key: sg.Kind, Label: kindLabel(sg.Kind)}
	for i, sl := range ge.Slots {
		if strings.HasPrefix(sl.Key, "saves") && strings.TrimSpace(ge.Paths[sl.Key]) == "" && !slotUsedElsewhere(g, thisDevice, emuID, sl.Key) {
			ge.Slots[i] = slot
			delete(ge.Paths, sl.Key)
			ge.Paths[sg.Kind] = sg.Path
			return nil
		}
	}
	ge.Slots = append(ge.Slots, slot)
	ge.Paths[sg.Kind] = sg.Path
	return nil
}

func kindLabel(kind string) string {
	if kind == saveKindDevice {
		return "Device save"
	}
	return "Account save"
}

// slotUsedElsewhere reports whether another device has a folder for slot key
// of the emulator profile emuID.
func slotUsedElsewhere(g library.Game, thisDevice, emuID, key string) bool {
	for id, d := range g.Devices {
		if id != thisDevice && strings.TrimSpace(d.Emulators[emuID].Paths[key]) != "" {
			return true
		}
	}
	return false
}

// uniqueSlotKey is "saves-<n>", or the next number that no slot has.
func uniqueSlotKey(slots []library.SaveSlot, n int) string {
	for {
		key := fmt.Sprintf("saves-%d", n)
		if !slices.ContainsFunc(slots, func(sl library.SaveSlot) bool { return sl.Key == key }) {
			return key
		}
		n++
	}
}
