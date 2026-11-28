package kit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrandIconsParse(t *testing.T) {
	for name, icon := range map[string]any{
		"Steam": IconSteam, "Heroic": IconHeroic, "Lutris": IconLutris,
		"Google Drive": IconGoogleDrive, "Dropbox": IconDropbox,
	} {
		assert.NotNil(t, icon, name)
	}
}
