package services

import (
	"fmt"
	"slices"
	"strings"

	"github.com/akfaiz/carryover/internal/saves/emumanifest"
	"github.com/akfaiz/carryover/internal/saves/yuzu"
)

// Slot kinds of a Switch game's saves.
const (
	kindAccount = "account" // the save of a user profile: progress, settings
	kindDevice  = "device"  // shared by all users, e.g. ghost data
)

// yuzuSuggestions suggests the save folders of a Switch game in a yuzu-family
// emulator (Eden, Citron, Suyu, yuzu) as the exact folders on this device.
//
// The manifest names the saves as nand/user/save/0000000000000000/*/<title id>,
// where * is the profile ID. Syncing that would store the profile ID in the
// cloud, and another device, whose emulator made a different profile, would get
// an "orphaned profile". So the title folder itself is the location, with the
// profile's own ID in this device's path only: the account save and the device
// save become two locations, each stored flat, identical on every device.
//
// ok is false if the entry has no title ID (the caller uses the manifest's
// paths). expanded is the emulator's data folder with path tokens resolved,
// rootSlash the same as the user wrote it.
func yuzuSuggestions(e emumanifest.Entry, rootSlash, expanded, emulator string) (sgs []EmuSuggestion, note string, ok bool) {
	titleID := strings.ToUpper(romTitleID.FindString(strings.Join(e.Files, "\n")))
	if titleID == "" {
		return nil, "", false
	}
	onDisk := yuzu.Saves(expanded, titleID)

	// The profiles this emulator has. Save folders of any other profile are
	// orphans (an earlier sync brought them from another device): never suggested.
	var accounts, orphans []string
	if ids, err := yuzu.ProfileIDs(expanded); err == nil && len(ids) > 0 {
		accounts = ids
		for _, p := range onDisk {
			if p != yuzu.DeviceProfile && !slices.Contains(ids, p) {
				orphans = append(orphans, p)
			}
		}
	} else {
		for _, p := range onDisk {
			if p != yuzu.DeviceProfile {
				accounts = append(accounts, p)
			}
		}
	}

	folder := func(profile string) string {
		return rootSlash + "/nand/user/save/0000000000000000/" + profile + "/" + titleID
	}
	for _, p := range accounts {
		label := e.Title() + " (account save)"
		if len(accounts) > 1 {
			label = fmt.Sprintf("%s (account save, profile %s…)", e.Title(), p[:8])
		}
		sgs = append(sgs, EmuSuggestion{Game: label, Path: folder(p), Kind: kindAccount, Exists: slices.Contains(onDisk, p)})
	}
	sgs = append(sgs, EmuSuggestion{
		Game: e.Title() + " (device save)", Path: folder(yuzu.DeviceProfile), Kind: kindDevice,
		Exists: slices.Contains(onDisk, yuzu.DeviceProfile),
	})

	switch {
	case len(accounts) == 0:
		note = emulator + " has no user profile on this device yet. Start it once, or enter the account save folder yourself."
	case len(orphans) > 0:
		note = fmt.Sprintf("%s has save folders of a profile it doesn't have (%s…), for example from an earlier sync. They are not suggested.", emulator, orphans[0][:8])
	}
	return sgs, note, true
}
