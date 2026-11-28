package msg_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/msg"
)

func TestErrorsKeepTheirTextAndChain(t *testing.T) {
	sentinel := msg.SyncLocked.Err()
	wrapped := msg.SessionPlayingHere.Wrap(sentinel)

	require.ErrorIs(t, wrapped, sentinel)
	assert.Equal(t, "being played on this device: game is already running or syncing in another Carryover process", wrapped.Error())
	require.ErrorIs(t, msg.SyncScanFailed.Wrap(fs.ErrNotExist), fs.ErrNotExist)
}

func TestOfMatchesTheErrorsText(t *testing.T) {
	coded := msg.CloudOffline.Err()
	for name, err := range map[string]error{
		"coded":          coded,
		"wrapping":       msg.SessionUploadFailed.Wrap(errors.New("quota")),
		"prefixed":       fmt.Errorf("step 2: %w", coded),
		"suffixed":       fmt.Errorf("%w: timeout", coded),
		"two errors":     fmt.Errorf("%w: %w", coded, msg.CloudNoAnswer.Err("Time", "5s")),
		"somewhere else": fmt.Errorf("(%w)", coded),
		"plain":          errors.New("disk full"),
	} {
		assert.Equal(t, err.Error(), msg.Of(err).Text, name)
	}
	assert.True(t, msg.Of(nil).IsZero())
	assert.Nil(t, msg.Of(errors.New("disk full")).Ptr(), "nothing to translate")
}

func TestPlural(t *testing.T) {
	assert.Equal(t, "Uploaded 1 file", msg.SyncDoneUploaded.Msg(msg.CountArg, 1).Text)
	assert.Equal(t, "Uploaded 3 files", msg.SyncDoneUploaded.Msg(msg.CountArg, 3).Text)
	assert.Equal(t, "Uploaded 3 files, deleted 1 file from cloud",
		msg.SyncDoneParts.Msg("Parts", msg.List{msg.SyncDoneUploaded.Msg(msg.CountArg, 3), msg.SyncAndDeletedCloud.Msg(msg.CountArg, 1)}).Text)
}

// Saved notices and sync states come back from JSON: the message must survive.
func TestMessageSurvivesJSON(t *testing.T) {
	m := msg.SessionUploadFailed.Wrap(msg.SyncUploadFailed.Wrap(errors.New("quota"), "Slot", "saves")).Message()
	data, err := json.Marshal(m)
	require.NoError(t, err)
	var back msg.Message
	require.NoError(t, json.Unmarshal(data, &back))

	assert.Equal(t, m, back)

	listed := msg.SyncDoneParts.Msg("Parts", msg.List{msg.SyncDoneUploaded.Msg(msg.CountArg, 3), msg.SyncAndDeletedCloud.Msg(msg.CountArg, 1)})
	data, err = json.Marshal(listed)
	require.NoError(t, err)
	var again msg.Message
	require.NoError(t, json.Unmarshal(data, &again))
	assert.Equal(t, listed, again)
	assert.Equal(t, m, msg.FromSaved(m.Text, &back))
	assert.Equal(t, msg.Raw("older data"), msg.FromSaved("older data", nil))
	assert.Equal(t, msg.Raw("changed since"), msg.FromSaved("changed since", &back))
}
