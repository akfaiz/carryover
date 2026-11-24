package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrefsPersist(t *testing.T) {
	dir := t.TempDir()
	p := loadPrefs(dir)
	assert.True(t, p.BoolWithFallback("window.fullscreen", true), "fallback while unset")
	p.SetBool("window.fullscreen", false)
	assert.False(t, loadPrefs(dir).BoolWithFallback("window.fullscreen", true), "saved value wins, even false")
}

func TestPrefsStrings(t *testing.T) {
	dir := t.TempDir()
	p := loadPrefs(dir)
	assert.Equal(t, "name", p.StringWithFallback("sort", "name"))
	p.SetString("sort", "recent")
	assert.Equal(t, "recent", loadPrefs(dir).StringWithFallback("sort", "name"))
	p.SetString("sort", "") // an empty value is a value
	assert.Equal(t, "", loadPrefs(dir).StringWithFallback("sort", "name"))
}
