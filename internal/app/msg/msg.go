// Package msg holds the text the core shows to users in a form the apps can
// translate: each text is defined once in catalog.go with a stable code and its
// English template. A Message carries the code, the values of the template's
// {{.Name}} placeholders and the whole English text, which logs, the command
// line and an app that doesn't know the code show as it is.
package msg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Code names a message. Codes are part of what the apps read (and of saved
// state), so keep them stable.
type Code string

// List is a value made of several messages, shown with ", " between them.
type List []Message

func (l List) join(text func(Message) string) string {
	parts := make([]string, len(l))
	for i, m := range l {
		parts[i] = text(m)
	}
	return strings.Join(parts, ", ")
}

// String is the English text.
func (l List) String() string { return l.join(Message.String) }

// CountArg is the placeholder that picks a plural message's form.
const CountArg = "Count"

// Def is a message of the catalog.
type Def struct {
	Code Code
	// One is the English text for a Count of 1; "" for a message without plural forms.
	One string
	// Other is the English template ("{{.Name}}" placeholders).
	Other string
}

var catalog = map[Code]Def{}

func define(code Code, text string) Def { return register(Def{Code: code, Other: text}) }

func plural(code Code, one, other string) Def {
	return register(Def{Code: code, One: one, Other: other})
}

func register(d Def) Def {
	if _, ok := catalog[d.Code]; ok {
		panic("msg: code defined twice: " + string(d.Code))
	}
	catalog[d.Code] = d
	return d
}

// Lookup returns the definition of a code.
func Lookup(code Code) (Def, bool) {
	d, ok := catalog[code]
	return d, ok
}

// Defs returns the whole catalog, sorted by code.
func Defs() []Def {
	out := make([]Def, 0, len(catalog))
	for _, d := range catalog {
		out = append(out, d)
	}
	slices.SortFunc(out, func(a, b Def) int { return strings.Compare(string(a.Code), string(b.Code)) })
	return out
}

// Message is one text to show. The zero Message is no text.
type Message struct {
	// Code is the catalog entry; "" for text that has none (an error of rclone
	// or the system, which stays in its own language).
	Code Code `json:"code,omitempty"`
	// Args are the values of the template's placeholders: strings, integers
	// and Lists.
	Args map[string]any `json:"args,omitempty"`
	// Cause is what this message is about, shown after it and ": ".
	Cause *Message `json:"cause,omitempty"`
	// Text is the whole message in English, cause included.
	Text string `json:"text"`
}

// String is the English text.
func (m Message) String() string { return m.Text }

// UnmarshalJSON reads a saved message back with Args of the types it was made
// with: JSON alone would turn counts into floats and Lists into plain slices.
func (m *Message) UnmarshalJSON(data []byte) error {
	type plain Message // without this method
	var in struct {
		plain
		Args map[string]json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	*m = Message(in.plain)
	m.Args = nil
	for k, raw := range in.Args {
		v, err := argValue(raw)
		if err != nil {
			return fmt.Errorf("msg: argument %q: %w", k, err)
		}
		if m.Args == nil {
			m.Args = make(map[string]any, len(in.Args))
		}
		m.Args[k] = v
	}
	return nil
}

func argValue(raw json.RawMessage) (any, error) {
	if bytes.HasPrefix(raw, []byte("[")) {
		var l List
		err := json.Unmarshal(raw, &l)
		return l, err
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n, nil
	}
	var s string
	err := json.Unmarshal(raw, &s)
	return s, err
}

// IsZero reports whether there is no text.
func (m Message) IsZero() bool { return m.Code == "" && m.Text == "" }

// Ptr is the message for an optional JSON field: nil unless it, or its cause,
// has a code an app could translate.
func (m Message) Ptr() *Message {
	if !m.Translatable() {
		return nil
	}
	return &m
}

// Translatable reports whether the message or one of its causes has a code.
func (m Message) Translatable() bool {
	return m.Code != "" || m.Cause != nil && m.Cause.Translatable()
}

// Translate returns the message in another language. format returns the text of
// a code with its values; ok is false for a code it doesn't know, whose English
// text is then used.
func (m Message) Translate(format func(code Code, args map[string]any) (text string, ok bool)) string {
	if !m.Translatable() {
		return m.Text
	}
	if m.Code == "" {
		// Untranslatable words in front of a translatable cause.
		if lead, ok := strings.CutSuffix(m.Text, ": "+m.Cause.Text); ok {
			return lead + ": " + m.Cause.Translate(format)
		}
		return m.Text
	}
	text, ok := format(m.Code, m.translatedArgs(format))
	if !ok {
		return m.Text
	}
	if m.Cause != nil {
		text += ": " + m.Cause.Translate(format)
	}
	return text
}

// translatedArgs is Args with every List put together in the language.
func (m Message) translatedArgs(format func(Code, map[string]any) (string, bool)) map[string]any {
	var out map[string]any
	for k, v := range m.Args {
		if l, ok := v.(List); ok {
			if out == nil {
				out = maps.Clone(m.Args)
			}
			out[k] = l.join(func(item Message) string { return item.Translate(format) })
		}
	}
	if out == nil {
		return m.Args
	}
	return out
}

// Count reads a Count value.
func Count(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case string:
		i, err := strconv.Atoi(n)
		return i, err == nil
	}
	return 0, false
}

// Msg is the message with the values of its placeholders, given as pairs of a
// name and a value.
func (d Def) Msg(kv ...any) Message {
	var args map[string]any
	if len(kv) > 0 {
		args = make(map[string]any, len(kv)/2)
		for i := 0; i+1 < len(kv); i += 2 {
			v := kv[i+1]
			switch v.(type) {
			case string, int, int64, List:
			default:
				v = fmt.Sprint(v)
			}
			args[fmt.Sprint(kv[i])] = v
		}
	}
	return Message{Code: d.Code, Args: args, Text: d.render(args)}
}

func (d Def) render(args map[string]any) string {
	text := d.Other
	if d.One != "" {
		if n, ok := Count(args[CountArg]); ok && n == 1 {
			text = d.One
		}
	}
	for k, v := range args {
		text = strings.ReplaceAll(text, "{{."+k+"}}", fmt.Sprint(v))
	}
	return text
}

// Err is an error with this message.
func (d Def) Err(kv ...any) *Error { return &Error{msg: d.Msg(kv...)} }

// Wrap is an error with this message about err, which it wraps: its text is the
// message, ": " and err's.
func (d Def) Wrap(err error, kv ...any) *Error { return &Error{msg: d.Msg(kv...), err: err} }

// Error is an error whose text the apps can translate. Make one with Def.Err or
// Def.Wrap.
type Error struct {
	msg Message
	err error
}

func (e *Error) Error() string {
	if e.err != nil {
		return e.msg.Text + ": " + e.err.Error()
	}
	return e.msg.Text
}

// Unwrap returns the wrapped error, if any.
func (e *Error) Unwrap() error { return e.err }

// Message is the error's text, with its cause.
func (e *Error) Message() Message {
	m := e.msg
	if e.err != nil {
		cause := Of(e.err)
		m.Cause, m.Text = &cause, e.Error()
	}
	return m
}

// Raw is text without a code.
func Raw(text string) Message { return Message{Text: text} }

// Of is the message of an error: that of the Error it is, or wraps at the start
// or the end of its text (as fmt.Errorf's "%w" does), else its text as it is.
// A nil error is no message.
func Of(err error) Message {
	if err == nil {
		return Message{}
	}
	if e, ok := err.(*Error); ok { //nolint:errorlint // a wrapped one is handled below
		return e.Message()
	}
	text := err.Error()
	if two, ok := err.(interface{ Unwrap() []error }); ok { //nolint:errorlint // this error itself
		// fmt.Errorf("%w: %w", a, b)
		if parts := two.Unwrap(); len(parts) == 2 && text == parts[0].Error()+": "+parts[1].Error() {
			return Of(parts[0]).followedBy(Of(parts[1]))
		}
	}
	var e *Error
	if !errors.As(err, &e) {
		return Raw(text)
	}
	inner := e.Message()
	if text == inner.Text {
		return inner
	}
	if rest, ok := strings.CutPrefix(text, inner.Text+": "); ok {
		return inner.followedBy(Raw(rest))
	}
	if strings.HasSuffix(text, ": "+inner.Text) {
		return Message{Cause: &inner, Text: text}
	}
	return Raw(text)
}

// followedBy appends next at the end of the chain of causes.
func (m Message) followedBy(next Message) Message {
	cause := next
	if m.Cause != nil {
		cause = m.Cause.followedBy(next)
	}
	m.Cause, m.Text = &cause, m.Text+": "+next.Text
	return m
}

// FromSaved is a message saved as its English text and, since this version,
// the message itself (nil in older data).
func FromSaved(text string, m *Message) Message {
	if m != nil && m.Text == text {
		return *m
	}
	return Raw(text)
}
