package game

import (
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
)

// serviceBackend is the Backend of the application: the services behind env.Env.
type serviceBackend struct{ u env.Env }

func (b serviceBackend) Plan(key string) (services.PlanView, error) { return b.u.Sync().Plan(key) }
func (b serviceBackend) Sync(key, resolution string) (string, error) {
	return b.u.Sync().Sync(key, resolution)
}
func (b serviceBackend) CancelSync(key string) bool { return b.u.Sync().Cancel(key) }
func (b serviceBackend) Backups(key string) ([]syncer.Backup, error) {
	return b.u.Sync().Backups(key)
}
func (b serviceBackend) BackupNow(key string) error { return b.u.Sync().BackupNow(key) }
func (b serviceBackend) DeleteBackup(key, name string) error {
	return b.u.Sync().DeleteBackup(key, name)
}
func (b serviceBackend) SetBackupPinned(key, name string, pinned bool) error {
	return b.u.Sync().SetBackupPinned(key, name, pinned)
}
func (b serviceBackend) RestoreBackup(key, name string) error {
	return b.u.Sync().RestoreBackup(key, name)
}
func (b serviceBackend) Play(gameID string) error   { return b.u.Sess().Play(gameID) }
func (b serviceBackend) CancelPlay(key string) bool { return b.u.Sess().Cancel(key) }
func (b serviceBackend) Delete(gameID string, withSaves bool) error {
	if withSaves {
		return b.u.Lib().DeleteWithSaves(gameID)
	}
	return b.u.Lib().Delete(gameID)
}
func (b serviceBackend) SteamInfo() services.SteamInfo { return b.u.Steam().Info() }
func (b serviceBackend) WrapLaunchOptions(gameID string) (string, error) {
	return b.u.Steam().WrapLaunchOptions(gameID)
}
func (b serviceBackend) AddToSteam(key, userID, mode string) (services.AddToSteamResult, error) {
	return b.u.Steam().AddToSteam(key, userID, mode)
}
func (b serviceBackend) RemoveFromSteam(key, userID string) (int, error) {
	return b.u.Steam().RemoveFromSteam(key, userID)
}
