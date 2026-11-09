package steam

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Binary VDF value types used in shortcuts.vdf.
const (
	TypeMap    byte = 0x00
	TypeString byte = 0x01
	TypeInt32  byte = 0x02
	typeEnd    byte = 0x08
)

// BinNode is a node in a binary VDF document. Order of children is preserved.
type BinNode struct {
	Type     byte
	Key      string
	Str      string
	Int      uint32
	Children []*BinNode
}

// Child returns the first child with the key (case-insensitive), or nil.
func (n *BinNode) Child(key string) *BinNode {
	for _, c := range n.Children {
		if strings.EqualFold(c.Key, key) {
			return c
		}
	}
	return nil
}

// GetString returns a child string value (or "").
func (n *BinNode) GetString(key string) string {
	if c := n.Child(key); c != nil && c.Type == TypeString {
		return c.Str
	}
	return ""
}

// GetInt returns a child int value (or 0).
func (n *BinNode) GetInt(key string) uint32 {
	if c := n.Child(key); c != nil && c.Type == TypeInt32 {
		return c.Int
	}
	return 0
}

// SetString sets or adds a string child.
func (n *BinNode) SetString(key, v string) {
	if c := n.Child(key); c != nil {
		c.Type, c.Str = TypeString, v
		return
	}
	n.Children = append(n.Children, &BinNode{Type: TypeString, Key: key, Str: v})
}

// SetInt sets or adds an int child.
func (n *BinNode) SetInt(key string, v uint32) {
	if c := n.Child(key); c != nil {
		c.Type, c.Int = TypeInt32, v
		return
	}
	n.Children = append(n.Children, &BinNode{Type: TypeInt32, Key: key, Int: v})
}

// ParseBinary parses a binary VDF document. The returned root is a map node
// with an empty key containing the top-level entries.
func ParseBinary(data []byte) (*BinNode, error) {
	r := bufio.NewReader(bytes.NewReader(data))
	root := &BinNode{Type: TypeMap}
	if err := readMap(r, root, true); err != nil {
		return nil, err
	}
	return root, nil
}

func readCString(r *bufio.Reader) (string, error) {
	s, err := r.ReadString(0)
	if err != nil {
		return "", err
	}
	return s[:len(s)-1], nil
}

func readMap(r *bufio.Reader, parent *BinNode, top bool) error {
	for {
		t, err := r.ReadByte()
		if errors.Is(err, io.EOF) && top {
			return nil
		}
		if err != nil {
			return err
		}
		if t == typeEnd {
			return nil
		}
		key, err := readCString(r)
		if err != nil {
			return err
		}
		n := &BinNode{Type: t, Key: key}
		switch t {
		case TypeMap:
			if err := readMap(r, n, false); err != nil {
				return err
			}
		case TypeString:
			if n.Str, err = readCString(r); err != nil {
				return err
			}
		case TypeInt32:
			var b [4]byte
			if _, err := io.ReadFull(r, b[:]); err != nil {
				return err
			}
			n.Int = binary.LittleEndian.Uint32(b[:])
		default:
			return fmt.Errorf("unsupported binary vdf type 0x%02x for key %q", t, key)
		}
		parent.Children = append(parent.Children, n)
	}
}

// MarshalBinary encodes the children of root as a binary VDF document.
func MarshalBinary(root *BinNode) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeChildren(&buf, root); err != nil {
		return nil, err
	}
	buf.WriteByte(typeEnd)
	return buf.Bytes(), nil
}

func writeChildren(buf *bytes.Buffer, n *BinNode) error {
	for _, c := range n.Children {
		if strings.IndexByte(c.Key, 0) >= 0 || strings.IndexByte(c.Str, 0) >= 0 {
			return errors.New("vdf: NUL byte in key or value")
		}
		buf.WriteByte(c.Type)
		buf.WriteString(c.Key)
		buf.WriteByte(0)
		switch c.Type {
		case TypeMap:
			if err := writeChildren(buf, c); err != nil {
				return err
			}
			buf.WriteByte(typeEnd)
		case TypeString:
			buf.WriteString(c.Str)
			buf.WriteByte(0)
		case TypeInt32:
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], c.Int)
			buf.Write(b[:])
		default:
			return fmt.Errorf("vdf: unsupported type 0x%02x", c.Type)
		}
	}
	return nil
}
