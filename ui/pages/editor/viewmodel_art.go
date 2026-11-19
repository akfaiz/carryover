package editor

import "github.com/akfaiz/carryover/services"

// artEdit is the editor's pending state for one kind of artwork.
type artEdit struct {
	path    string // stored file, "" if none
	version int64
	src     string            // local image chosen by the user, copied on save
	pick    *services.ArtGrid // SteamGridDB image, downloaded on save
	clear   bool              // remove the stored image on save
}

func (a *artEdit) changed() bool { return a.src != "" || a.pick != nil || a.clear }

// has reports whether the kind will have an image after saving.
func (a *artEdit) has() bool {
	if a.clear {
		return false
	}
	return a.src != "" || a.pick != nil || a.path != ""
}

func (e *ViewModel) artEdit(kind services.ArtKind) *artEdit {
	if e.art == nil {
		e.art = map[services.ArtKind]*artEdit{}
	}
	a := e.art[kind]
	if a == nil {
		a = &artEdit{}
		e.art[kind] = a
	}
	return a
}

// artworkJobs lists what to change on save, for every kind with a pending change.
func (e *ViewModel) artworkJobs() map[services.ArtKind]artEdit {
	jobs := map[services.ArtKind]artEdit{}
	for k, a := range e.art {
		if a.changed() {
			jobs[k] = *a
		}
	}
	return jobs
}
