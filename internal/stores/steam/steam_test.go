package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	got, err := ParseBinary(b)
	require.NoError(t, err)
	b2, err := MarshalBinary(got)
	require.NoError(t, err)
	require.True(t, bytes.Equal(b, b2), "round trip mismatch")
	sc := got.Child("shortcuts").Children[0]
	assert.EqualValues(t, 0x80001234, sc.GetInt("appid"))
	assert.Equal(t, "Game A", sc.GetString("appname"))
}

func TestShortcutsUpsertRemove(t *testing.T) {
	root := t.TempDir()
	user := "12345"
	match := func(s Shortcut) bool { return s.LaunchOptions == "run g1" }
	id, err := UpsertShortcut(root, user, Shortcut{AppName: "Game A", Exe: `"/usr/bin/carryover"`, LaunchOptions: "run g1", Tags: []string{"Carryover"}}, match)
	require.NoError(t, err)
	require.NotZero(t, id&0x80000000)
	// Upsert again replaces rather than duplicates.
	_, err = UpsertShortcut(root, user, Shortcut{AppName: "Game A2", Exe: `"/usr/bin/carryover"`, LaunchOptions: "run g1"}, match)
	require.NoError(t, err)
	_, err = UpsertShortcut(root, user, Shortcut{AppName: "Other", Exe: `"/x"`}, func(Shortcut) bool { return false })
	require.NoError(t, err)
	list, err := ListShortcuts(root, user)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "Game A2", list[0].AppName)
	n, err := RemoveShortcuts(root, user, match)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	list, _ = ListShortcuts(root, user)
	require.Len(t, list, 1)
	require.Equal(t, "Other", list[0].AppName)
	require.FileExists(t, filepath.Join(root, "userdata", user, "config", "shortcuts.vdf.carryover.bak"), "backup not written")
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
	require.NoError(t, err)
	lf := doc.Child("libraryfolders")
	require.Equal(t, `D:\SteamLibrary`, lf.Child("1").Get("path"))
}

func TestInstalledApps(t *testing.T) {
	root := t.TempDir()
	steamapps := filepath.Join(root, "steamapps")
	require.NoError(t, os.MkdirAll(steamapps, 0o755))
	write := func(id, name, dir string) {
		acf := "\"AppState\"\n{\n\t\"appid\"\t\t\"" + id + "\"\n\t\"name\"\t\t\"" + name + "\"\n\t\"installdir\"\t\t\"" + dir + "\"\n}\n"
		require.NoError(t, os.WriteFile(filepath.Join(steamapps, "appmanifest_"+id+".acf"), []byte(acf), 0o644))
	}
	write("367520", "Hollow Knight", "Hollow Knight")
	write("1493710", "Proton Experimental", "Proton - Experimental")
	write("228980", "Steamworks Common Redistributables", "Steamworks Shared")
	write("105600", "Terraria", "Terraria")
	apps := InstalledApps(root)
	require.Len(t, apps, 2)
	require.Equal(t, "Hollow Knight", apps[0].Name)
	require.Equal(t, "105600", apps[1].AppID)
	require.Equal(t, filepath.Join(steamapps, "common", "Hollow Knight"), apps[0].InstallDir)
}

func TestSetGridArt(t *testing.T) {
	root, src := t.TempDir(), filepath.Join(t.TempDir(), "a.jpg")
	require.NoError(t, os.WriteFile(src, []byte("img"), 0o644))
	dir := GridDir(root, "42")
	for kind, name := range map[string]string{
		"cover": "100p.jpg", "wide": "100.jpg", "hero": "100_hero.jpg", "logo": "100_logo.jpg", "icon": "100_icon.jpg",
	} {
		// A stale file of another extension must be replaced, not left beside it.
		stale := filepath.Join(dir, strings.TrimSuffix(name, ".jpg")+".png")
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(stale, []byte("old"), 0o644)
		dst, err := SetGridArt(root, "42", 100, kind, src)
		require.NoError(t, err, kind)
		require.Equal(t, name, filepath.Base(dst), kind)
		assert.NoFileExists(t, stale, "%s: stale file kept", kind)
	}
	_, err := SetGridArt(root, "42", 100, "bogus", src)
	require.Error(t, err, "unknown kind accepted")
}

func TestUsers(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"0", "abc", "67890", "12345", "99999999999"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "userdata", d), 0o755))
	}
	// A plain file in userdata is not an account.
	require.NoError(t, os.WriteFile(filepath.Join(root, "userdata", "11111"), nil, 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "config"), 0o755))
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
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", "loginusers.vdf"), []byte(vdf), 0o644))

	users, err := Users(root)
	require.NoError(t, err)
	// Sorted by name, so the account shown by its id comes first.
	want := []User{{ID: "67890", Name: "67890"}, {ID: "12345", Name: "Alice"}}
	require.Len(t, users, len(want))
	for i := range want {
		require.Equal(t, want[i], users[i])
	}
}

func TestUsersErrors(t *testing.T) {
	_, err := Users("")
	assert.Error(t, err, "Users without a Steam root")
	_, err = Users(t.TempDir())
	assert.Error(t, err, "Users without a userdata folder")
}

// Without login names (or with a broken file) accounts keep their id as name.
func TestLoginUserNamesTolerant(t *testing.T) {
	root := t.TempDir()
	require.Empty(t, loginUserNames(root), "missing file")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "config"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", "loginusers.vdf"), []byte(`"users" {`), 0o644))
	require.Empty(t, loginUserNames(root), "broken file")
}

func TestFindRootEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARRYOVER_STEAM_ROOT", dir)
	require.Equal(t, dir, FindRoot())
}

func TestShortcutAppID(t *testing.T) {
	a := ShortcutAppID(`"/opt/carryover"`, "Game A")
	require.NotZero(t, a&0x80000000)
	require.Equal(t, ShortcutAppID(`"/opt/carryover"`, "Game A"), a, "not deterministic")
	require.NotEqual(t, ShortcutAppID(`"/opt/carryover"`, "Game B"), a, "different games share an app id")
}

func TestCompatDataPrefixFallsBackToRoot(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "steamapps", "compatdata", "42", "pfx", "drive_c", "users", "steamuser")
	require.Equal(t, want, CompatDataPrefix(root, "42"))
	require.NoError(t, os.MkdirAll(want, 0o755))
	require.Equal(t, want, CompatDataPrefix(root, "42"))
}

// IsRunning depends on the machine; it must just answer without failing.
func TestIsRunningDoesNotPanic(t *testing.T) { _ = IsRunning() }
