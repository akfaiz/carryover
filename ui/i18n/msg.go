package i18n

import (
	"errors"

	"github.com/akfaiz/carryover/internal/app/msg"
)

// The texts of the core (internal/app/msg) are translated from the same
// catalogs: a message's English template is its key, so every template of
// msg.Defs is listed in translations/en.json (TestTranslationCatalog).

// Msg translates a text of the core. A code this catalog lacks, and text
// without a code (an error of rclone or the system), stay in English.
func Msg(m msg.Message) string { return m.Translate(coreText) }

// Text is the text of a view field and its message: m translated, or english
// when there is no message.
func Text(english string, m *msg.Message) string {
	if m == nil {
		return english
	}
	return Msg(*m)
}

// Err is the translated text of an error, "" for nil.
func Err(err error) string { return Msg(msg.Of(err)) }

// Translated is err with its text translated, for a widget that shows an
// error's text itself. It is nil for nil.
func Translated(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(Err(err))
}

func coreText(code msg.Code, args map[string]any) (string, bool) {
	def, ok := msg.Lookup(code)
	if !ok {
		return "", false
	}
	var data []any
	if args != nil {
		data = []any{args}
	}
	var count any
	if def.One != "" {
		if n, ok := msg.Count(args[msg.CountArg]); ok {
			count = n
		}
	}
	return localize(def.Other, count, data), true
}
