// Package yuzu finds a game's save folders in the data folder of a yuzu-family
// Switch emulator (Eden, Citron, Suyu, yuzu).
//
// Such an emulator stores a game's save under
//
//	nand/user/save/0000000000000000/<profile id>/<title id>
//
// where <profile id> is the ID of the user profile that owns the save (all
// zeros for the device save, shared by every user). Every installation creates
// its own random profile ID, so the profile folder must not be part of what is
// synced: Carryover syncs the title folder itself and each device uses its own
// profile's path. The profiles an installation has are listed in
// nand/system/save/8000000000000010/su/avators/profiles.dat.
package yuzu

import (
	"encoding/hex"
	"errors"
	"io"
	"strings"

	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
)

// DeviceProfile is the folder name of the device save, shared by all profiles.
const DeviceProfile = "00000000000000000000000000000000"

const (
	profilesHeader = 0x10 // padding before the first profile
	profileSize    = 56   // 16-byte ID, 8-byte timestamp, 32-byte name
	maxProfiles    = 8
)

// SaveRoot is the folder that holds the profile folders. dataDir may be a
// folder without a file path (a Storage Access Framework folder on Android):
// every read here goes through localfs.
func SaveRoot(dataDir string) string { return localfs.Join(dataDir, SaveRootRel) }

// SaveRootRel and ProfilesRel are SaveRoot and the profiles file relative to
// the data folder, '/'-separated (for data folders without a file path).
const (
	SaveRootRel = "nand/user/save/0000000000000000"
	ProfilesRel = "nand/system/save/8000000000000010/su/avators/profiles.dat"
)

// profilesFile is the emulator's list of user profiles.
func profilesFile(dataDir string) string { return localfs.Join(dataDir, ProfilesRel) }

// ParseProfiles returns the profile IDs in a profiles.dat, as the names of
// their save folders (32 upper-case hex digits). The file stores each ID
// byte-reversed.
func ParseProfiles(b []byte) ([]string, error) {
	if len(b) < profilesHeader+maxProfiles*profileSize {
		return nil, errors.New("profiles.dat is too short")
	}
	var ids []string
	for i := range maxProfiles {
		raw := b[profilesHeader+i*profileSize : profilesHeader+i*profileSize+16]
		if allZero(raw) {
			continue
		}
		id := make([]byte, 16)
		for j := range 16 {
			id[j] = raw[15-j]
		}
		ids = append(ids, strings.ToUpper(hex.EncodeToString(id)))
	}
	return ids, nil
}

// EncodeProfiles builds a profiles.dat for the given profile IDs (the inverse of
// ParseProfiles, with empty names), for tests and tools.
func EncodeProfiles(ids []string) []byte {
	b := make([]byte, profilesHeader+maxProfiles*profileSize+8*144)
	for i, s := range ids {
		raw, err := hex.DecodeString(s)
		if err != nil || len(raw) != 16 || i >= maxProfiles {
			continue
		}
		for j := range 16 {
			b[profilesHeader+i*profileSize+j] = raw[15-j]
		}
	}
	return b
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// ProfileIDs lists the user profiles of the emulator in dataDir.
func ProfileIDs(dataDir string) ([]string, error) {
	r, err := localfs.Open(profilesFile(dataDir))
	if err != nil {
		return nil, msg.PathYuzuProfiles.Wrap(err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, msg.PathYuzuProfiles.Wrap(err)
	}
	return ParseProfiles(b)
}

// IsProfileID reports whether s looks like a profile folder name.
func IsProfileID(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// TitleDir is the folder holding a game's save of one profile.
func TitleDir(dataDir, profileID, titleID string) string {
	return localfs.Join(SaveRoot(dataDir), strings.ToUpper(profileID)+"/"+strings.ToUpper(titleID))
}

// Saves lists the profile folders under the save root that hold a save of the
// game, device save (DeviceProfile) included.
func Saves(dataDir, titleID string) []string {
	root := SaveRoot(dataDir)
	entries, err := localfs.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.Dir || !IsProfileID(e.Name) {
			continue
		}
		if st, err := localfs.Stat(localfs.Join(root, e.Name+"/"+strings.ToUpper(titleID))); err == nil && st.Dir {
			out = append(out, strings.ToUpper(e.Name))
		}
	}
	return out
}
