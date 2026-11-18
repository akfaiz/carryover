package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormat(t *testing.T) {
	assert.Equal(t, "never", TimeAgo(""))
}
