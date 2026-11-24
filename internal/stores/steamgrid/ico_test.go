package steamgrid

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// icoEntry is one image of a test icon: its size, bit depth and bytes.
type icoEntry struct {
	w, h, bits int
	data       []byte
}

// buildICO lays the entries out as an .ico file.
func buildICO(entries ...icoEntry) []byte {
	var b bytes.Buffer
	le := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	le(uint16(0))
	le(uint16(1))
	le(uint16(len(entries)))
	off := 6 + 16*len(entries)
	for _, e := range entries {
		b.WriteByte(byte(e.w % 256))
		b.WriteByte(byte(e.h % 256))
		b.WriteByte(0)
		b.WriteByte(0)
		le(uint16(1))
		le(uint16(e.bits))
		le(uint32(len(e.data)))
		le(uint32(off))
		off += len(e.data)
	}
	for _, e := range entries {
		b.Write(e.data)
	}
	return b.Bytes()
}

// dib is an icon bitmap: header, palette, the rows (bottom first) as row(y)
// gives them, and the transparency mask with the pixels opaque(x, y) says no to.
func dib(w, h, bits int, palette []color.NRGBA, row func(y int) []byte, opaque func(x, y int) bool) []byte {
	var b bytes.Buffer
	le := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	le(uint32(40))
	le(int32(w))
	le(int32(2 * h))
	le(uint16(1))
	le(uint16(bits))
	le(uint32(0))
	le(uint32(0))
	le(int32(0))
	le(int32(0))
	le(uint32(len(palette)))
	le(uint32(0))
	for _, p := range palette {
		b.Write([]byte{p.B, p.G, p.R, 0})
	}
	stride := (w*bits + 31) / 32 * 4
	for y := h - 1; y >= 0; y-- {
		r := make([]byte, stride)
		copy(r, row(y))
		b.Write(r)
	}
	maskStride := (w + 31) / 32 * 4
	for y := h - 1; y >= 0; y-- {
		r := make([]byte, maskStride)
		for x := range w {
			if !opaque(x, y) {
				r[x/8] |= 0x80 >> (x % 8)
			}
		}
		b.Write(r)
	}
	return b.Bytes()
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	require.True(t, bytes.HasPrefix(data, pngMagic), "the result is a PNG")
	m, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	return m
}

func nrgba(m image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(m.At(x, y)).(color.NRGBA)
}

func TestICOPicksTheLargestImageAndKeepsItsPNG(t *testing.T) {
	big := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	var p bytes.Buffer
	require.NoError(t, png.Encode(&p, big))
	small := dib(16, 16, 24, nil, func(int) []byte { return nil }, func(int, int) bool { return true })

	out, ext, err := convertImage(buildICO(icoEntry{16, 16, 24, small}, icoEntry{256, 256, 32, p.Bytes()}))
	require.NoError(t, err)
	assert.Equal(t, ".png", ext)
	assert.Equal(t, p.Bytes(), out, "the 256 px PNG inside is taken as it is")
}

func TestICO32BitUsesItsAlpha(t *testing.T) {
	// Left half opaque red, right half half-transparent blue.
	row := func(int) []byte {
		r := make([]byte, 0, 16)
		r = append(r, 0, 0, 255, 255, 0, 0, 255, 255) // BGRA red, red
		r = append(r, 255, 0, 0, 128, 255, 0, 0, 128) // BGRA blue, blue
		return r
	}
	d := dib(4, 2, 32, nil, row, func(int, int) bool { return true })
	out, err := icoToPNG(buildICO(icoEntry{4, 2, 32, d}))
	require.NoError(t, err)
	m := decodePNG(t, out)
	assert.Equal(t, image.Rect(0, 0, 4, 2), m.Bounds())
	assert.Equal(t, color.NRGBA{255, 0, 0, 255}, nrgba(m, 0, 1))
	assert.Equal(t, color.NRGBA{0, 0, 255, 128}, nrgba(m, 3, 0))
}

func TestICO24BitUsesTheMaskAndRowsGoBottomUp(t *testing.T) {
	// Top row green, bottom row white; the top-left pixel is masked out.
	row := func(y int) []byte {
		if y == 0 {
			return []byte{0, 255, 0, 0, 255, 0}
		}
		return []byte{255, 255, 255, 255, 255, 255}
	}
	d := dib(2, 2, 24, nil, row, func(x, y int) bool { return x != 0 || y != 0 })
	out, err := icoToPNG(buildICO(icoEntry{2, 2, 24, d}))
	require.NoError(t, err)
	m := decodePNG(t, out)
	assert.Equal(t, uint8(0), nrgba(m, 0, 0).A, "masked out")
	assert.Equal(t, color.NRGBA{0, 255, 0, 255}, nrgba(m, 1, 0))
	assert.Equal(t, color.NRGBA{255, 255, 255, 255}, nrgba(m, 0, 1))
}

func TestICOPaletteImage(t *testing.T) {
	pal := []color.NRGBA{{0, 0, 0, 255}, {200, 100, 50, 255}}
	d := dib(8, 1, 1, pal, func(int) []byte { return []byte{0b01000000} }, func(int, int) bool { return true })
	out, err := icoToPNG(buildICO(icoEntry{8, 1, 1, d}))
	require.NoError(t, err)
	m := decodePNG(t, out)
	assert.Equal(t, color.NRGBA{0, 0, 0, 255}, nrgba(m, 0, 0))
	assert.Equal(t, color.NRGBA{200, 100, 50, 255}, nrgba(m, 1, 0))
}

func TestICOBrokenFilesFail(t *testing.T) {
	_, err := icoToPNG([]byte{0, 0, 1, 0, 1, 0})
	require.Error(t, err, "an entry that is not there")
	bad := buildICO(icoEntry{16, 16, 32, []byte("short")})
	_, _, err = convertImage(bad)
	require.Error(t, err)
	_, _, err = convertImage([]byte("GIF89a......"))
	require.Error(t, err)
}
