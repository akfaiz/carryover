package steam

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// TextNode is a node of a text (KeyValues) VDF document. A node either has a
// Value (leaf) or Children (section).
type TextNode struct {
	Key      string
	Value    string
	Children []*TextNode
	IsMap    bool
}

// Child returns the first child with key (case-insensitive), or nil.
func (n *TextNode) Child(key string) *TextNode {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if strings.EqualFold(c.Key, key) {
			return c
		}
	}
	return nil
}

// Get returns the string value of a child leaf, or "".
func (n *TextNode) Get(key string) string {
	if c := n.Child(key); c != nil {
		return c.Value
	}
	return ""
}

// ParseText parses a text VDF document and returns a synthetic root node.
func ParseText(src string) (*TextNode, error) {
	p := &textParser{s: src}
	root := &TextNode{IsMap: true}
	if err := p.parseBody(root, true); err != nil {
		return nil, err
	}
	return root, nil
}

type textParser struct {
	s   string
	pos int
}

func (p *textParser) skip() {
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if unicode.IsSpace(rune(c)) {
			p.pos++
			continue
		}
		if strings.HasPrefix(p.s[p.pos:], "//") {
			if i := strings.IndexByte(p.s[p.pos:], '\n'); i >= 0 {
				p.pos += i + 1
			} else {
				p.pos = len(p.s)
			}
			continue
		}
		break
	}
}

func (p *textParser) token() (tok string, quoted bool, err error) {
	p.skip()
	if p.pos >= len(p.s) {
		return "", false, errors.New("unexpected end of input")
	}
	c := p.s[p.pos]
	if c == '{' || c == '}' {
		p.pos++
		return string(c), false, nil
	}
	if c == '"' {
		p.pos++
		var b strings.Builder
		for p.pos < len(p.s) {
			c := p.s[p.pos]
			if c == '\\' && p.pos+1 < len(p.s) {
				n := p.s[p.pos+1]
				switch n {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case '\\', '"':
					b.WriteByte(n)
				default:
					b.WriteByte('\\')
					b.WriteByte(n)
				}
				p.pos += 2
				continue
			}
			if c == '"' {
				p.pos++
				return b.String(), true, nil
			}
			b.WriteByte(c)
			p.pos++
		}
		return "", false, errors.New("unterminated string")
	}
	start := p.pos
	for p.pos < len(p.s) && !unicode.IsSpace(rune(p.s[p.pos])) && p.s[p.pos] != '{' && p.s[p.pos] != '}' && p.s[p.pos] != '"' {
		p.pos++
	}
	return p.s[start:p.pos], false, nil
}

func (p *textParser) parseBody(parent *TextNode, top bool) error {
	for {
		p.skip()
		if p.pos >= len(p.s) {
			if top {
				return nil
			}
			return errors.New("unexpected end of input in section")
		}
		key, quoted, err := p.token()
		if err != nil {
			return err
		}
		if key == "}" && !quoted {
			if top {
				return errors.New("unexpected '}'")
			}
			return nil
		}
		val, vquoted, err := p.token()
		if err != nil {
			return err
		}
		node := &TextNode{Key: key}
		if val == "{" && !vquoted {
			node.IsMap = true
			if err := p.parseBody(node, false); err != nil {
				return fmt.Errorf("in %q: %w", key, err)
			}
		} else {
			node.Value = val
		}
		parent.Children = append(parent.Children, node)
	}
}
