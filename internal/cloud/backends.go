package cloud

// rclone backends compiled into PlayRelay. Keep this list short: every
// backend adds to the binary size. Update Supported to match.
import (
	"slices"

	_ "github.com/rclone/rclone/backend/drive"
	_ "github.com/rclone/rclone/backend/dropbox"
	_ "github.com/rclone/rclone/backend/local" // required: save folders are read through it
	_ "github.com/rclone/rclone/backend/onedrive"
	_ "github.com/rclone/rclone/backend/sftp"
	_ "github.com/rclone/rclone/backend/smb"
	_ "github.com/rclone/rclone/backend/webdav"
)

// Supported lists the storage providers offered to users, in display order.
// The local backend is compiled in but deliberately not offered.
var Supported = []string{"drive", "dropbox", "onedrive", "webdav", "sftp", "smb"}

// IsSupported reports whether remotes of this type can be used.
func IsSupported(typ string) bool {
	return slices.Contains(Supported, typ)
}
