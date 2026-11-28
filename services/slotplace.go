package services

import (
	"net/url"
	"slices"
	"strings"

	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
)

// A save location is shared: every device syncs its own folder with the same
// cloud folder, through the same filters. So a suggested folder may only go
// where it holds the same files as on the other devices, or the saves of one
// kind are downloaded among those of another.

// SlotState is what decides whether a suggested folder can go into a save
// location.
type SlotState struct {
	Include []string // its filters, shared by every device
	Path    string   // this device's folder, "" if it has none
	// Elsewhere are the folders other devices have for it.
	Elsewhere []string
}

// UsedElsewhere reports whether another device has a folder for the location:
// its filters are then not this device's to change.
func (s SlotState) UsedElsewhere() bool {
	return slices.ContainsFunc(s.Elsewhere, func(p string) bool { return strings.TrimSpace(p) != "" })
}

// TakesFilters reports whether filling the location with a suggestion sets the
// suggestion's filters on it: only when it has none and no other device uses it.
func (s SlotState) TakesFilters() bool { return !s.UsedElsewhere() && len(s.Include) == 0 }

// PlaceSuggestion picks the save location of slots that a suggested folder
// (path with its filters) belongs in, among those with no folder on this device.
// index is that location, or -1 if none matches. mayAdd then reports whether
// the folder can be a new location of its own. It can't while a location
// another device uses is still empty here: the folder may be that one's, under
// another name, which only the user knows.
//
// A location another device uses matches when a folder there has the same name
// and the filters don't rule each other out. One that no other device uses
// matches when its filters don't, and comes after those. Between equal matches,
// the one whose parent folder has the same name too wins (RetroArch's saves and
// states folders each have one per core).
func PlaceSuggestion(slots []SlotState, path string, include []string) (index int, mayAdd bool) {
	const (
		unused      = 1 // nobody has a folder for it
		sameFolder  = 2 // another device's folder has the name; one side has no filters
		sameFiles   = 4 // and the filters name some of the same files
		parentBonus = 1 // between equals, the parent folder decides
	)
	name, parent := folderNames(path)
	index, mayAdd = -1, true
	best := 0
	for i, s := range slots {
		if strings.TrimSpace(s.Path) != "" {
			continue
		}
		shared := slices.ContainsFunc(s.Include, func(f string) bool { return slices.Contains(include, f) })
		compatible := shared || len(s.Include) == 0 || len(include) == 0
		score := 0
		switch {
		case !s.UsedElsewhere():
			if compatible {
				score = unused
			}
		default:
			mayAdd = false
			named, parentToo := false, false
			for _, other := range s.Elsewhere {
				if n, p := folderNames(other); strings.EqualFold(n, name) {
					named, parentToo = true, parentToo || strings.EqualFold(p, parent)
				}
			}
			switch {
			case !named || !compatible:
			case shared:
				score = sameFiles
			default:
				score = sameFolder
			}
			if score > 0 && parentToo {
				score += parentBonus
			}
		}
		if score > best {
			index, best = i, score
		}
	}
	return index, mayAdd
}

// folderNames are the last two folder names of a save path of any device (the
// parent "" if there is none): a Windows or Unix path, with or without tokens,
// or a folder granted on Android, whose names are inside the escaped document
// id.
func folderNames(p string) (name, parent string) {
	p = strings.TrimSpace(p)
	if rest, ok := strings.CutPrefix(p, localfs.SAF); ok {
		if plain, err := url.PathUnescape(rest); err == nil {
			rest = plain
		}
		p = strings.ReplaceAll(rest, ":", "/") // "primary:PSP"
	}
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	if n := len(parts); n > 0 {
		name = parts[n-1]
		if n > 1 {
			parent = parts[n-2]
		}
	}
	return name, parent
}
