package steamgrid

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// pngMagic starts every PNG file, and the PNG images inside an icon.
var pngMagic = []byte("\x89PNG\r\n\x1a\n")

// icoToPNG returns the largest image of a Windows icon (.ico) as a PNG. Many
// of SteamGridDB's icons are .ico files, which neither the app's image
// decoders nor every Steam client read. An icon holds one or more images,
// each a PNG (the usual case for the 256 px one) or an uncompressed bitmap
// with a 1-bit transparency mask after it.
func icoToPNG(data []byte) ([]byte, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:]) != 0 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return nil, errors.New("not an icon file")
	}
	n := int(binary.LittleEndian.Uint16(data[4:]))
	best, bestArea, bestBits := -1, 0, 0
	for i := range n {
		e := 6 + 16*i
		if e+16 > len(data) {
			break
		}
		w, h := int(data[e]), int(data[e+1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		bits := int(binary.LittleEndian.Uint16(data[e+6:]))
		if area := w * h; area > bestArea || area == bestArea && bits > bestBits {
			best, bestArea, bestBits = i, area, bits
		}
	}
	if best < 0 {
		return nil, errors.New("icon has no images")
	}
	e := 6 + 16*best
	size := int(binary.LittleEndian.Uint32(data[e+8:]))
	off := int(binary.LittleEndian.Uint32(data[e+12:]))
	if off < 0 || size <= 0 || off > len(data) || size > len(data)-off {
		return nil, errors.New("icon image out of bounds")
	}
	img := data[off : off+size]
	if bytes.HasPrefix(img, pngMagic) {
		return img, nil
	}
	m, err := decodeIconBitmap(img)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeIconBitmap decodes an icon's bitmap image: a BITMAPINFOHEADER, a
// palette for 8 bits or fewer, the colors bottom row first and then the
// transparency mask (the header's height counts both).
func decodeIconBitmap(b []byte) (image.Image, error) {
	if len(b) < 40 {
		return nil, errors.New("icon bitmap too short")
	}
	hdr := int(binary.LittleEndian.Uint32(b[0:]))
	w := int(int32(binary.LittleEndian.Uint32(b[4:])))
	h := int(int32(binary.LittleEndian.Uint32(b[8:]))) / 2
	bits := int(binary.LittleEndian.Uint16(b[14:]))
	compression := binary.LittleEndian.Uint32(b[16:])
	colors := int(binary.LittleEndian.Uint32(b[32:]))
	if hdr < 40 || hdr > len(b) || w <= 0 || h <= 0 || w > 1024 || h > 1024 {
		return nil, errors.New("icon bitmap has a bad header")
	}
	if compression != 0 && (compression != 3 || bits != 32) { // BI_RGB, or BI_BITFIELDS read as BGRA
		return nil, fmt.Errorf("icon bitmap compression %d not supported", compression)
	}
	var palette []color.NRGBA
	switch bits {
	case 1, 4, 8:
		if colors == 0 || colors > 1<<bits {
			colors = 1 << bits
		}
		p := hdr
		if compression == 3 {
			p += 12
		}
		if p+4*colors > len(b) {
			return nil, errors.New("icon palette out of bounds")
		}
		for i := range colors {
			q := p + 4*i
			palette = append(palette, color.NRGBA{R: b[q+2], G: b[q+1], B: b[q], A: 255})
		}
	case 24, 32:
	default:
		return nil, fmt.Errorf("icon bitmap with %d bits per pixel not supported", bits)
	}
	pix := hdr + 4*len(palette)
	if compression == 3 {
		pix += 12
	}
	stride := (w*bits + 31) / 32 * 4
	maskStride := (w + 31) / 32 * 4
	mask := pix + stride*h
	if pix+stride*h > len(b) {
		return nil, errors.New("icon bitmap out of bounds")
	}
	hasMask := mask+maskStride*h <= len(b)
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	anyAlpha := false
	for y := range h {
		row := b[pix+(h-1-y)*stride:]
		for x := range w {
			var c color.NRGBA
			switch bits {
			case 32:
				c = color.NRGBA{R: row[4*x+2], G: row[4*x+1], B: row[4*x], A: row[4*x+3]}
				anyAlpha = anyAlpha || c.A != 0
			case 24:
				c = color.NRGBA{R: row[3*x+2], G: row[3*x+1], B: row[3*x], A: 255}
			default:
				bit := x * bits
				i := int(row[bit/8]>>(8-bits-bit%8)) & (1<<bits - 1)
				if i < len(palette) {
					c = palette[i]
				}
			}
			m.SetNRGBA(x, y, c)
		}
	}
	// The mask says which pixels are transparent, unless 32-bit colors carry
	// their own alpha (an icon whose alpha is all zero uses the mask instead).
	if bits != 32 || !anyAlpha {
		for y := range h {
			for x := range w {
				transparent := false
				if hasMask {
					row := b[mask+(h-1-y)*maskStride:]
					transparent = row[x/8]&(0x80>>(x%8)) != 0
				}
				c := m.NRGBAAt(x, y)
				c.A = 255
				if transparent {
					c.A = 0
				}
				m.SetNRGBA(x, y, c)
			}
		}
	}
	return m, nil
}
