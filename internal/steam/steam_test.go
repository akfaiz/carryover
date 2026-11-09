package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBinaryRoundTrip(t *testing.T) {
	doc := &BinNode{Type: TypeMap}
	list := &BinNode{Type: TypeMap, Key: "shortcuts"}
	doc.Children = append(doc.Children, list)
	e := &BinNode{Type: TypeMap, Key: "0"}
	e.SetInt("appid", 0x80001234)
	e.SetString("AppName", "Game A")
	e.SetString("Exe", `"C:\Games\A\a.exe"`)
	tags := &BinNode{Type: TypeMap, Key: "tags"}
	tags.Children = append(tags.Children, &BinNode{Type: TypeString, Key: "0", Str: "PlayRelay"})
	e.Children = append(e.Children, tags)
	list.Children = append(list.Children, e)

	b, err := MarshalBinary(doc)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseBinary(b)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := MarshalBinary(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, b2) {
		t.Fatal("round trip mismatch")
	}
	sc := got.Child("shortcuts").Children[0]
	if sc.GetInt("appid") != 0x80001234 || sc.GetString("appname") != "Game A" {
		t.Fatalf("bad parse: %+v", sc)
	}
}

func TestShortcutsUpsertRemove(t *testing.T) {
	root := t.TempDir()
	user := "12345"
	match := func(s Shortcut) bool { return s.LaunchOptions == "run g1" }
	id, err := UpsertShortcut(root, user, Shortcut{AppName: "Game A", Exe: `"/usr/bin/playrelay"`, LaunchOptions: "run g1", Tags: []string{"PlayRelay"}}, match)
	if err != nil {
		t.Fatal(err)
	}
	if id&0x80000000 == 0 {
		t.Fatalf("appid high bit not set: %x", id)
	}
	// Upsert again replaces rather than duplicates.
	if _, err := UpsertShortcut(root, user, Shortcut{AppName: "Game A2", Exe: `"/usr/bin/playrelay"`, LaunchOptions: "run g1"}, match); err != nil {
		t.Fatal(err)
	}
	if _, err := UpsertShortcut(root, user, Shortcut{AppName: "Other", Exe: `"/x"`}, func(Shortcut) bool { return false }); err != nil {
		t.Fatal(err)
	}
	list, err := ListShortcuts(root, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].AppName != "Game A2" {
		t.Fatalf("unexpected shortcuts: %+v", list)
	}
	n, err := RemoveShortcuts(root, user, match)
	if err != nil || n != 1 {
		t.Fatalf("remove = %d, %v", n, err)
	}
	list, _ = ListShortcuts(root, user)
	if len(list) != 1 || list[0].AppName != "Other" {
		t.Fatalf("after remove: %+v", list)
	}
	if _, err := os.Stat(filepath.Join(root, "userdata", user, "config", "shortcuts.vdf.playrelay.bak")); err != nil {
		t.Fatal("backup not written")
	}
}

func TestTextVDF(t *testing.T) {
	src := `"libraryfolders"
{
	"0"
	{
		"path"		"C:\\Program Files (x86)\\Steam"
		"apps" { "228980" "1" }
	}
	"1"
	{
		"path"		"D:\\SteamLibrary"
	}
}`
	doc, err := ParseText(src)
	if err != nil {
		t.Fatal(err)
	}
	lf := doc.Child("libraryfolders")
	if lf.Child("1").Get("path") != `D:\SteamLibrary` {
		t.Fatalf("got %q", lf.Child("1").Get("path"))
	}
}

func TestInstalledApps(t *testing.T) {
	root := t.TempDir()
	steamapps := filepath.Join(root, "steamapps")
	if err := os.MkdirAll(steamapps, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(id, name, dir string) {
		acf := "\"AppState\"\n{\n\t\"appid\"\t\t\"" + id + "\"\n\t\"name\"\t\t\"" + name + "\"\n\t\"installdir\"\t\t\"" + dir + "\"\n}\n"
		if err := os.WriteFile(filepath.Join(steamapps, "appmanifest_"+id+".acf"), []byte(acf), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("367520", "Hollow Knight", "Hollow Knight")
	write("1493710", "Proton Experimental", "Proton - Experimental")
	write("228980", "Steamworks Common Redistributables", "Steamworks Shared")
	write("105600", "Terraria", "Terraria")
	apps := InstalledApps(root)
	if len(apps) != 2 || apps[0].Name != "Hollow Knight" || apps[1].AppID != "105600" {
		t.Fatalf("apps = %+v", apps)
	}
	if apps[0].InstallDir != filepath.Join(steamapps, "common", "Hollow Knight") {
		t.Fatalf("install dir = %q", apps[0].InstallDir)
	}
}
