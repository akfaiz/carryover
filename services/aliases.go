package services

import (
	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/launcher"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/stores/steamgrid"
	"github.com/akfaiz/carryover/internal/stores/steamstore"
)

// What the GUI needs from the packages under internal/, re-exported so that
// ui/ depends on services only and a change down there stays out of the UI.

// Types.
type (
	// ArtKind is a kind of artwork (cover, wide capsule, hero, logo, icon).
	ArtKind = steamgrid.Kind
	// ArtGrid is one image of SteamGridDB.
	ArtGrid = steamgrid.Grid
	// ArtGame is a game of SteamGridDB.
	ArtGame = steamgrid.Game
	// StoreMatch is a Steam store search result.
	StoreMatch = steamstore.Match
	// LaunchMode is how a game is started (direct, Steam wrap, Heroic).
	LaunchMode = library.LaunchMode
	// SaveSlot is one save location of a game.
	SaveSlot = library.SaveSlot
	// AddToSteamResult is what adding a shortcut to Steam did.
	AddToSteamResult = core.AddToSteamResult
	// Message is a text of the core that the apps translate (see
	// internal/app/msg); views carry one next to its English text.
	Message = msg.Message
)

// Artwork kinds.
const (
	ArtCover ArtKind = steamgrid.KindCover
	ArtWide  ArtKind = steamgrid.KindWide
	ArtHero  ArtKind = steamgrid.KindHero
	ArtLogo  ArtKind = steamgrid.KindLogo
	ArtIcon  ArtKind = steamgrid.KindIcon
)

// ArtKinds are the kinds beyond the cover, in display order.
var ArtKinds = steamgrid.ArtKinds

// Launch modes.
const (
	LaunchDirect    = library.LaunchDirect
	LaunchSteamWrap = library.LaunchSteamWrap
	LaunchHeroic    = library.LaunchHeroic
	LaunchLutris    = library.LaunchLutris
	LaunchBottles   = library.LaunchBottles
)

// NativeProfile names the game's own launch config (no emulator).
const NativeProfile = core.NativeProfile

// SyncKey is the key a game's saves are synced under: the game id for the
// native profile, "<game>.<emulator>" for an emulator.
func SyncKey(gameID, emulatorID string) string { return core.SyncKey(gameID, emulatorID) }

// SelfArtKinds are the kinds of library art Carryover's own Steam shortcut
// gets, in the order they are written.
var SelfArtKinds = core.SelfArtKinds

// AppBundle returns the .app bundle a program picked inside one belongs to
// (macOS), or the path unchanged.
func AppBundle(path string) string { return launcher.AppBundle(path) }

// ErrorText is an error as its English text and, when the apps can translate
// it, its message (nil otherwise).
func ErrorText(err error) (string, *Message) {
	m := msg.Of(err)
	return m.Text, m.Ptr()
}

// ErrOffline is wrapped by errors meaning the cloud couldn't be reached.
var ErrOffline = core.ErrOffline

// IsOffline reports whether err means the cloud couldn't be reached (no
// internet), which is not a sync failure: saves stay on this device.
func IsOffline(err error) bool { return core.IsOffline(err) }
