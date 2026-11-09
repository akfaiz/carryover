package library

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndMerge(t *testing.T) {
	dir := t.TempDir()
	pc, _ := Open(filepath.Join(dir, "pc.json"))
	deck, _ := Open(filepath.Join(dir, "deck.json"))

	g, err := pc.Save(Game{
		Name:  "Game A",
		Slots: []SaveSlot{{Key: "saves"}},
		Devices: map[string]DeviceConfig{
			"pc": {Paths: map[string]string{"saves": "{appdata}/GameA"}, Exe: `C:\Games\A\a.exe`},
		},
	}, "pc")
	if err != nil {
		t.Fatal(err)
	}

	// Deck receives the catalog and configures its own paths.
	if changed, err := deck.MergeCatalog(pc.Catalog()); err != nil || !changed {
		t.Fatalf("merge: %v %v", changed, err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := deck.UpdateDevice(g.ID, "deck", func(d *DeviceConfig) {
		d.Paths["saves"] = "{proton:shortcut}/AppData/Roaming/GameA"
	}); err != nil {
		t.Fatal(err)
	}

	// PC merges deck's catalog: keeps its own config and gains deck's.
	if _, err := pc.MergeCatalog(deck.Catalog()); err != nil {
		t.Fatal(err)
	}
	got, _ := pc.Get(g.ID)
	if got.Devices["pc"].Exe != `C:\Games\A\a.exe` || got.Devices["deck"].Paths["saves"] == "" {
		t.Fatalf("merge lost device config: %+v", got.Devices)
	}

	// Saving on PC must not drop the deck's config.
	got.Name = "Game A (GOTY)"
	if _, err := pc.Save(got, "pc"); err != nil {
		t.Fatal(err)
	}
	got, _ = pc.Get(g.ID)
	if _, ok := got.Devices["deck"]; !ok {
		t.Fatal("deck config dropped on save")
	}

	// Deletion propagates as a tombstone.
	if err := pc.Delete(g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := deck.MergeCatalog(pc.Catalog()); err != nil {
		t.Fatal(err)
	}
	if len(deck.List()) != 0 {
		t.Fatal("deleted game still listed on deck")
	}

	// Reload from disk.
	again, err := Open(filepath.Join(dir, "deck.json"))
	if err != nil || len(again.Catalog().Games) != 1 {
		t.Fatalf("reload: %v", err)
	}
}

func TestValidateSlots(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "l.json"))
	if _, err := s.Save(Game{Name: "x", Slots: []SaveSlot{{Key: "a"}, {Key: "a"}}}, "d"); err == nil {
		t.Fatal("expected duplicate key error")
	}
	if _, err := s.Save(Game{Name: "x", Slots: []SaveSlot{{Key: "a/b"}}}, "d"); err == nil {
		t.Fatal("expected invalid key error")
	}
}
