package mobile

import (
	"encoding/json"

	"github.com/akfaiz/carryover/internal/app/msg"
)

// ErrorPrefix starts the text of an error the app can translate: what follows
// is the error as a msg.Message in JSON. Only an error's text reaches Kotlin,
// so the code and its values travel in it.
const ErrorPrefix = "carryover-msg:"

type wireError struct {
	err  error
	text string
}

func (e *wireError) Error() string { return e.text }

func (e *wireError) Unwrap() error { return e.err }

// wire puts the message of an error into its text, if it has a code. Every
// exported function that returns an error defers it (TestExportedErrorsAreWired).
func wire(err *error) {
	if *err == nil {
		return
	}
	m := msg.Of(*err)
	if !m.Translatable() {
		return
	}
	b, jsonErr := json.Marshal(m)
	if jsonErr != nil {
		return
	}
	*err = &wireError{err: *err, text: ErrorPrefix + string(b)}
}
