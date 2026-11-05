package platform

import (
	"bytes"
	"encoding/binary"
	"unicode/utf16"
)

var shellLinkCLSID = []byte{0x01, 0x14, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}

// BuildShortcut creates a minimal .lnk pointing at target. Jigglypuff uses it for
// its own tests and the demo PC.
func BuildShortcut(target, args string) []byte {
	var out bytes.Buffer
	flags := uint32(0x2 | 0x80)
	if args != "" {
		flags |= 0x20
	}
	hdr := make([]byte, 0x4C)
	binary.LittleEndian.PutUint32(hdr[0:], 0x4C)
	copy(hdr[4:], shellLinkCLSID)
	binary.LittleEndian.PutUint32(hdr[20:], flags)
	binary.LittleEndian.PutUint32(hdr[24:], 0x20)
	binary.LittleEndian.PutUint32(hdr[60:], 1)
	out.Write(hdr)

	ansi := []byte(target)
	for i, c := range ansi {
		if c > 0x7f {
			ansi[i] = '?'
		}
	}
	wide := utf16.Encode([]rune(target))
	volID := []byte{0x11, 0, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0x10, 0, 0, 0, 0}
	const hdrSize = 0x24
	baseOff := hdrSize + len(volID)
	suffixOff := baseOff + len(ansi) + 1
	baseUOff := suffixOff + 1
	suffixUOff := baseUOff + len(wide)*2 + 2
	total := suffixUOff + 2
	li := make([]byte, total)
	binary.LittleEndian.PutUint32(li[0:], uint32(total))
	binary.LittleEndian.PutUint32(li[4:], hdrSize)
	binary.LittleEndian.PutUint32(li[8:], 1)
	binary.LittleEndian.PutUint32(li[12:], hdrSize)
	binary.LittleEndian.PutUint32(li[16:], uint32(baseOff))
	binary.LittleEndian.PutUint32(li[24:], uint32(suffixOff))
	binary.LittleEndian.PutUint32(li[28:], uint32(baseUOff))
	binary.LittleEndian.PutUint32(li[32:], uint32(suffixUOff))
	copy(li[hdrSize:], volID)
	copy(li[baseOff:], ansi)
	for i, c := range wide {
		binary.LittleEndian.PutUint16(li[baseUOff+i*2:], c)
	}
	out.Write(li)
	if args != "" {
		a := utf16.Encode([]rune(args))
		n := make([]byte, 2+len(a)*2)
		binary.LittleEndian.PutUint16(n, uint16(len(a)))
		for i, c := range a {
			binary.LittleEndian.PutUint16(n[2+i*2:], c)
		}
		out.Write(n)
	}
	out.Write([]byte{0, 0, 0, 0})
	return out.Bytes()
}
