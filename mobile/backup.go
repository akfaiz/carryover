package mobile

// Backups of a game's saves: the zips Carryover keeps in the cloud before it
// overwrites saves (when turned on in the settings), the ones made on request,
// and the copies this device keeps before a download replaces its saves.
// key is a profile key (a game id, or "<game>.<emulator>").

// Backups lists the backups of profile key, newest first, as a JSON array of
// syncer.Backup (name, time, deviceName, kind, pinned, local). It needs the
// cloud.
func Backups(key string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.sync.Backups(key))
}

// BackupNow stores a pinned backup of this device's current saves of profile
// key, whether or not automatic backups are on. It returns a SyncOutcome as
// JSON (action "none"): being offline or cancelled (Cancel(key)) is not an
// error.
func BackupNow(key string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return stopped(s.sync.BackupNow(key))
}

// SetBackupPinned pins or unpins a backup; pinned backups are never deleted
// automatically.
func SetBackupPinned(key, name string, pinned bool) (err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return err
	}
	return s.sync.SetBackupPinned(key, name, pinned)
}

// DeleteBackup deletes a backup for good, for every device.
func DeleteBackup(key, name string) (err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return err
	}
	return s.sync.DeleteBackup(key, name)
}

// RestoreBackup replaces the saves on this device, and then in the cloud, with
// a backup. The saves on this device are backed up first. It returns a
// SyncOutcome as JSON (action "none"): being offline or cancelled
// (Cancel(key)) is not an error.
func RestoreBackup(key, name string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return stopped(s.sync.RestoreBackup(key, name))
}

// stopped turns the result of a cancellable call into a SyncOutcome.
func stopped(err error) (string, error) {
	o := SyncOutcome{Action: "none"}
	o.Offline, o.Cancelled, err = outcome(err)
	return encode(o, err)
}
