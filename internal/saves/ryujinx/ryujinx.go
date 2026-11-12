// Package ryujinx finds a game's save folder inside Ryujinx's data folder.
//
// Ryujinx stores saves in numbered folders (bis/user/save/0000000000000003)
// that don't name the game. Which folder belongs to which game is recorded in
// an index file, bis/system/save/8000000000000000/0/imkvdb.arc. Reading it lets
// Carryover sync one game's save instead of every game's.
package ryujinx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Kind is the type of a game's save data.
type Kind string

const (
	// Account is the save of a user profile (progress, settings).
	Account Kind = "account"
	// Device is shared by all users of the console (for example ghost data).
	Device Kind = "device"
)

// saveType is the index's save data type number.
func (k Kind) saveType() (byte, bool) {
	switch k {
	case Account:
		return 1, true
	case Device:
		return 3, true
	}
	return 0, false
}

// Entry is one record of the save index.
type Entry struct {
	ProgramID  uint64 // the game's title ID
	Type       byte   // 1 account, 3 device (others are system, cache, ...)
	UserID     [16]byte
	SaveDataID uint64 // names the folder under bis/user/save
}

// ErrNotFound means the game has no save of that kind in the index.
var ErrNotFound = errors.New("no save found in Ryujinx's save index")

const (
	indexMagic   = "IMKV"
	entryMagic   = "IMEN"
	headerSize   = 12 // magic, reserved, entry count
	entryHeader  = 12 // magic, key size, value size
	keyMinSize   = 33 // up to and including the type byte
	valueMinSize = 8  // the save data ID
)

// ParseIndex reads Ryujinx's save index (imkvdb.arc).
func ParseIndex(b []byte) ([]Entry, error) {
	if len(b) < headerSize || string(b[:4]) != indexMagic {
		return nil, errors.New("not a Ryujinx save index")
	}
	n := binary.LittleEndian.Uint32(b[8:])
	var out []Entry
	off := headerSize
	for range n {
		if off+entryHeader > len(b) || string(b[off:off+4]) != entryMagic {
			return nil, errors.New("save index is damaged")
		}
		ks := int(binary.LittleEndian.Uint32(b[off+4:]))
		vs := int(binary.LittleEndian.Uint32(b[off+8:]))
		k := off + entryHeader
		v := k + ks
		end := v + vs
		if ks < keyMinSize || vs < valueMinSize || end > len(b) {
			return nil, errors.New("save index is damaged")
		}
		e := Entry{ProgramID: binary.LittleEndian.Uint64(b[k:]), Type: b[k+32], SaveDataID: binary.LittleEndian.Uint64(b[v:])}
		copy(e.UserID[:], b[k+8:k+24])
		out = append(out, e)
		off = end
	}
	return out, nil
}

// EncodeIndex writes entries in the format of Ryujinx's save index: the IMKV
// header, then one IMEN record per entry with a 64-byte key (program, user,
// system save id, type) and a 64-byte value (save data id). Ryujinx writes the
// file itself; this is for tests and tools that need an index.
func EncodeIndex(entries []Entry) []byte {
	b := append([]byte(indexMagic), 0, 0, 0, 0)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(entries)))
	for _, e := range entries {
		b = append(b, entryMagic...)
		b = binary.LittleEndian.AppendUint32(b, 64)
		b = binary.LittleEndian.AppendUint32(b, 64)
		key := make([]byte, 64)
		binary.LittleEndian.PutUint64(key, e.ProgramID)
		copy(key[8:], e.UserID[:])
		key[32] = e.Type
		val := make([]byte, 64)
		binary.LittleEndian.PutUint64(val, e.SaveDataID)
		b = append(append(b, key...), val...)
	}
	return b
}

// indexDir is the system save that holds the index; 0 is the committed copy
// and 1 the working copy Ryujinx keeps next to it.
func indexDir(dataDir string) string {
	return filepath.Join(dataDir, "bis", "system", "save", "8000000000000000")
}

// ReadIndex reads the save index in a Ryujinx data folder.
func ReadIndex(dataDir string) ([]Entry, error) {
	var firstErr error
	for _, side := range []string{"0", "1"} {
		b, err := os.ReadFile(filepath.Join(indexDir(dataDir), side, "imkvdb.arc"))
		if err == nil {
			return ParseIndex(b)
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, fmt.Errorf("read Ryujinx's save index in %s: %w", dataDir, firstErr)
}

// ParseTitleID reads a 16-digit hexadecimal title ID.
func ParseTitleID(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if len(s) != 16 {
		return 0, fmt.Errorf("%q is not a title ID (16 hex digits)", s)
	}
	return strconv.ParseUint(s, 16, 64)
}

// SaveDir is the folder holding a game's save of a kind: the committed copy
// (".../0") of its numbered save folder. Ryujinx rebuilds its working copy
// (".../1") from it.
func SaveDir(dataDir, titleID string, kind Kind) (string, error) {
	want, ok := kind.saveType()
	if !ok {
		return "", fmt.Errorf("unknown save kind %q (use account or device)", kind)
	}
	id, err := ParseTitleID(titleID)
	if err != nil {
		return "", err
	}
	entries, err := ReadIndex(dataDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.ProgramID == id && e.Type == want {
			return filepath.Join(dataDir, "bis", "user", "save", fmt.Sprintf("%016X", e.SaveDataID), "0"), nil
		}
	}
	return "", fmt.Errorf("%w: the %s save of %016X (start the game once in Ryujinx)", ErrNotFound, kind, id)
}
