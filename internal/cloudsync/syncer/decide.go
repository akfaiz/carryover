package syncer

// Action is what a sync should do.
type Action string

const (
	ActionNone     Action = "none"
	ActionPush     Action = "push"
	ActionPull     Action = "pull"
	ActionConflict Action = "conflict"
)

// Decide chooses an action given the current local snapshot, the cloud
// manifest (nil if the cloud has nothing) and the manifest recorded at the last
// successful sync on this device (nil if never synced).
func Decide(local, remote, last *Manifest) Action {
	localEmpty := local == nil || len(local.Files) == 0
	if last == nil {
		switch {
		case remote == nil && localEmpty:
			return ActionNone
		case remote == nil:
			return ActionPush
		case localEmpty:
			return ActionPull
		case SameFiles(local, remote):
			return ActionNone
		default:
			return ActionConflict
		}
	}
	localChanged := !SameFiles(local, last)
	if remote == nil {
		// Cloud data vanished (or remote switched): re-upload what we have.
		if localEmpty {
			return ActionNone
		}
		return ActionPush
	}
	remoteChanged := remote.Revision != last.Revision
	switch {
	case !localChanged && !remoteChanged:
		return ActionNone
	case localChanged && !remoteChanged:
		return ActionPush
	case !localChanged && remoteChanged:
		if SameFiles(local, remote) {
			return ActionNone
		}
		return ActionPull
	default:
		if SameFiles(local, remote) {
			return ActionNone
		}
		return ActionConflict
	}
}
