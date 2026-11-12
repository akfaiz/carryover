package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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
	tags.Children = append(tags.Children, &BinNode{Type: TypeString, Key: "0", Str: "Carryover"})
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
	id, err := UpsertShortcut(root, user, Shortcut{AppName: "Game A", Exe: `"/usr/bin/carryover"`, LaunchOptions: "run g1", Tags: []string{"Carryover"}}, match)
	if err != nil {
		t.Fatal(err)
	}
	if id&0x80000000 == 0 {
		t.Fatalf("appid high bit not set: %x", id)
	}
	// Upsert again replaces rather than duplicates.
	if _, err := UpsertShortcut(root, user, Shortcut{AppName: "Game A2", Exe: `"/usr/bin/carryover"`, LaunchOptions: "run g1"}, match); err != nil {
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
	if _, err := os.Stat(filepath.Join(root, "userdata", user, "config", "shortcuts.vdf.carryover.bak")); err != nil {
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

func TestSetGridArt(t *testing.T) {
	root, src := t.TempDir(), filepath.Join(t.TempDir(), "a.jpg")
	if err := os.WriteFile(src, []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := GridDir(root, "42")
	for kind, name := range map[string]string{
		"cover": "100p.jpg", "wide": "100.jpg", "hero": "100_hero.jpg", "logo": "100_logo.jpg", "icon": "100_icon.jpg",
	} {
		// A stale file of another extension must be replaced, not left beside it.
		stale := filepath.Join(dir, strings.TrimSuffix(name, ".jpg")+".png")
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(stale, []byte("old"), 0o644)
		dst, err := SetGridArt(root, "42", 100, kind, src)
		if err != nil || filepath.Base(dst) != name {
			t.Fatalf("%s: %s, %v", kind, dst, err)
		}
		if _, err := os.Stat(stale); err == nil {
			t.Errorf("%s: stale %s kept", kind, stale)
		}
	}
	if _, err := SetGridArt(root, "42", 100, "bogus", src); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestUsers(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"0", "abc", "67890", "12345", "99999999999"} {
		if err := os.MkdirAll(filepath.Join(root, "userdata", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A plain file in userdata is not an account.
	if err := os.WriteFile(filepath.Join(root, "userdata", "11111"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	// SteamID64 = base + 12345 is named; 67890 has no entry and falls back to its id.
	vdf := `"users"
{
	"76561197960278073"
	{
		"AccountName"	"login"
		"PersonaName"	"Alice"
	}
	"1234"
	{
		"PersonaName"	"too small to be a SteamID64"
	}
}`
	if err := os.WriteFile(filepath.Join(root, "config", "loginusers.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}

	users, err := Users(root)
	if err != nil {
		t.Fatal(err)
	}
	// Sorted by name, so the account shown by its id comes first.
	want := []User{{ID: "67890", Name: "67890"}, {ID: "12345", Name: "Alice"}}
	if len(users) != len(want) {
		t.Fatalf("Users = %+v, want %+v", users, want)
	}
	for i := range want {
		if users[i] != want[i] {
			t.Fatalf("Users[%d] = %+v, want %+v", i, users[i], want[i])
		}
	}
}

func TestUsersErrors(t *testing.T) {
	if _, err := Users(""); err == nil {
		t.Error("Users without a Steam root: want an error")
	}
	if _, err := Users(t.TempDir()); err == nil {
		t.Error("Users without a userdata folder: want an error")
	}
}

// Without login names (or with a broken file) accounts keep their id as name.
func TestLoginUserNamesTolerant(t *testing.T) {
	root := t.TempDir()
	if got := loginUserNames(root); len(got) != 0 {
		t.Fatalf("missing file: %v", got)
	}
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "loginusers.vdf"), []byte(`"users" {`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loginUserNames(root); len(got) != 0 {
		t.Fatalf("broken file: %v", got)
	}
}

func TestFindRootEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARRYOVER_STEAM_ROOT", dir)
	if got := FindRoot(); got != dir {
		t.Fatalf("FindRoot = %q, want %q", got, dir)
	}
}

func TestShortcutAppID(t *testing.T) {
	a := ShortcutAppID(`"/opt/carryover"`, "Game A")
	if a&0x80000000 == 0 {
		t.Fatalf("high bit not set: %x", a)
	}
	if a != ShortcutAppID(`"/opt/carryover"`, "Game A") {
		t.Fatal("not deterministic")
	}
	if a == ShortcutAppID(`"/opt/carryover"`, "Game B") {
		t.Fatal("different games share an app id")
	}
}

func TestCompatDataPrefixFallsBackToRoot(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "steamapps", "compatdata", "42", "pfx", "drive_c", "users", "steamuser")
	if got := CompatDataPrefix(root, "42"); got != want {
		t.Fatalf("CompatDataPrefix = %q, want %q", got, want)
	}
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := CompatDataPrefix(root, "42"); got != want {
		t.Fatalf("existing prefix = %q, want %q", got, want)
	}
}

// IsRunning depends on the machine; it must just answer without failing.
func TestIsRunningDoesNotPanic(t *testing.T) { _ = IsRunning() }
