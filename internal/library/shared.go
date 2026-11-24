package library

import (
	"reflect"
	"slices"
	"time"
)

// A game's (or an emulator's) own UpdatedAt decides which copy's shared
// settings win a merge (MergeGame), while each device's entry is merged by
// its own UpdatedAt. So a save that changes only this device's entry must
// leave the shared timestamp alone: stamping it would let this copy's shared
// settings win even where they are older than another device's (a rename or a
// removed save slot made elsewhere and not merged here yet would be undone).

// sameSharedGame reports whether two versions of a game have the same shared
// settings: everything but the device entries and the timestamps.
func sameSharedGame(a, b Game) bool {
	return a.Deleted == b.Deleted && a.DeletedAt.Equal(b.DeletedAt) && reflect.DeepEqual(sharedGame(a), sharedGame(b))
}

func sharedGame(g Game) Game {
	g.Devices = nil
	g.CreatedAt, g.UpdatedAt, g.DeletedAt = time.Time{}, time.Time{}, time.Time{}
	if len(g.Art) == 0 {
		g.Art = nil
	}
	g.Slots = sharedSlots(g.Slots)
	if len(g.Emulators) == 0 {
		g.Emulators = nil
	} else {
		ems := make([]GameEmulator, len(g.Emulators))
		for i, e := range g.Emulators {
			e.Slots = sharedSlots(e.Slots)
			ems[i] = e
		}
		g.Emulators = ems
	}
	return g
}

// sharedSlots treats an empty list like no list, as JSON does not keep the
// difference ("[]" and "null" both come back as one or the other).
func sharedSlots(in []SaveSlot) []SaveSlot {
	if len(in) == 0 {
		return nil
	}
	out := make([]SaveSlot, len(in))
	for i, s := range in {
		if len(s.Include) == 0 {
			s.Include = nil
		}
		if len(s.Exclude) == 0 {
			s.Exclude = nil
		}
		out[i] = s
	}
	return out
}

// sameSharedEmulator is sameSharedGame for an emulator.
func sameSharedEmulator(a, b Emulator) bool {
	return a.Name == b.Name && a.Preset == b.Preset && slices.Equal(a.Platforms, b.Platforms) &&
		a.Deleted == b.Deleted && a.DeletedAt.Equal(b.DeletedAt)
}
