package library

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// twoPhones is a library on "pc" with a game and an emulator set up on pc and
// on two installs of the same phone, "old" and "new".
func twoPhones(t *testing.T) (*Store, string, string) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "pc.json"))
	require.NoError(t, err)
	g, err := s.Save(Game{
		Name:  "Game",
		Slots: []SaveSlot{{Key: "saves"}},
		Devices: map[string]DeviceConfig{
			"pc":  {DeviceName: "PC", Paths: map[string]string{"saves": "C:/saves"}},
			"old": {DeviceName: "Phone", Paths: map[string]string{"saves": "/old"}},
			"new": {DeviceName: "Phone", Paths: map[string]string{"saves": "/new"}},
		},
	}, "pc")
	require.NoError(t, err)
	other, err := s.Save(Game{
		Name:    "Other",
		Slots:   []SaveSlot{{Key: "saves"}},
		Devices: map[string]DeviceConfig{"old": {DeviceName: "Phone", Paths: map[string]string{"saves": "/other"}}},
	}, "old")
	require.NoError(t, err)
	e, err := s.SaveEmulator(Emulator{Name: "Eden", Devices: map[string]EmulatorDevice{
		"pc": {Exe: "eden.exe"}, "old": {Exe: "eden/old"},
	}}, "pc")
	require.NoError(t, err)
	_ = e
	return s, g.ID, other.ID
}

func TestDevicesListsEveryDevice(t *testing.T) {
	s, _, _ := twoPhones(t)
	devs := s.Devices()
	byID := map[string]DeviceInfo{}
	for _, d := range devs {
		byID[d.ID] = d
	}
	require.Len(t, devs, 3)
	assert.Equal(t, DeviceInfo{ID: "old", Name: "Phone", LastSeen: byID["old"].LastSeen, Games: 2, Emulators: 1}, byID["old"])
	assert.Equal(t, 1, byID["new"].Games)
	assert.Equal(t, 0, byID["new"].Emulators)
	assert.Equal(t, "PC", byID["pc"].Name)
	assert.True(t, s.HasDevice("old"))
	assert.False(t, s.HasDevice("nobody"))
}

func TestForgetDeviceSurvivesMerges(t *testing.T) {
	s, gameID, _ := twoPhones(t)
	// Another device still has the old phone in its copy of the catalog.
	deck, err := Open(filepath.Join(t.TempDir(), "deck.json"))
	require.NoError(t, err)
	_, err = deck.MergeCatalog(s.Catalog())
	require.NoError(t, err)

	require.NoError(t, s.ForgetDevice("old", time.Now().UTC()))
	assert.False(t, s.HasDevice("old"))
	c := s.Catalog()
	assert.Equal(t, CatalogVersion, c.Version, "a forgotten device needs the newer format")
	assert.Contains(t, c.ForgottenDevices, "old")

	// Merging the deck's stale copy doesn't bring it back.
	_, err = s.MergeCatalog(deck.Catalog())
	require.NoError(t, err)
	assert.False(t, s.HasDevice("old"))
	// And the deck drops it once it merges this catalog.
	changed, err := deck.MergeCatalog(s.Catalog())
	require.NoError(t, err)
	assert.True(t, changed)
	assert.False(t, deck.HasDevice("old"))

	// Reopened from disk, it stays forgotten.
	again, err := Open(s.path)
	require.NoError(t, err)
	assert.False(t, again.HasDevice("old"))
	g, err := again.Get(gameID)
	require.NoError(t, err)
	assert.Contains(t, g.Devices, "pc")
	assert.Contains(t, g.Devices, "new")
}

func TestForgottenDeviceComesBackWhenItChangesSomething(t *testing.T) {
	s, gameID, _ := twoPhones(t)
	require.NoError(t, s.ForgetDevice("old", time.Now().UTC().Add(-time.Minute)))
	_, err := s.UpdateDevice(gameID, "old", func(d *DeviceConfig) { d.Paths["saves"] = "/again" })
	require.NoError(t, err)
	g, err := s.Get(gameID)
	require.NoError(t, err)
	assert.Equal(t, "/again", g.Devices["old"].Paths["saves"])
}

func TestCatalogWithoutForgottenDevicesStaysVersion2(t *testing.T) {
	s, _, _ := twoPhones(t)
	assert.Equal(t, 2, s.Catalog().Version, "older clients keep reading it")
	assert.Empty(t, s.Catalog().ForgottenDevices)
}

func TestMoveDevice(t *testing.T) {
	s, gameID, otherID := twoPhones(t)
	// A copy made before the move, as another device would hold it.
	deck, err := Open(filepath.Join(t.TempDir(), "deck.json"))
	require.NoError(t, err)
	_, err = deck.MergeCatalog(s.Catalog())
	require.NoError(t, err)

	// The new install takes over the old one's identity.
	require.NoError(t, s.MoveDevice("new", "old", time.Now().UTC()))
	assert.False(t, s.HasDevice("new"))

	g, err := s.Get(gameID)
	require.NoError(t, err)
	assert.Equal(t, "/new", g.Devices["old"].Paths["saves"], "the newer settings win where both have some")
	other, err := s.Get(otherID)
	require.NoError(t, err)
	assert.Equal(t, "/other", other.Devices["old"].Paths["saves"], "the old install's settings are kept")
	e, err := s.FindEmulator("Eden")
	require.NoError(t, err)
	assert.Equal(t, "eden/old", e.Devices["old"].Exe)

	// The stale copy doesn't bring the new install's id back or undo the move.
	_, err = s.MergeCatalog(deck.Catalog())
	require.NoError(t, err)
	assert.False(t, s.HasDevice("new"))
	g, err = s.Get(gameID)
	require.NoError(t, err)
	assert.Equal(t, "/new", g.Devices["old"].Paths["saves"])
	_, err = deck.MergeCatalog(s.Catalog())
	require.NoError(t, err)
	g, err = deck.Get(gameID)
	require.NoError(t, err)
	assert.NotContains(t, g.Devices, "new")
	assert.Equal(t, "/new", g.Devices["old"].Paths["saves"])

	assert.ErrorIs(t, s.MoveDevice("old", "old", time.Now()), ErrUnknownDevice)
}
