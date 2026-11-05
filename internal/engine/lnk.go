package engine

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"jigglypuff/internal/platform"
)

// Shell link (.lnk) parsing, following Microsoft's [MS-SHLLINK] format.

const (
	lnkHasIDList     = 0x1
	lnkHasLinkInfo   = 0x2
	lnkHasName       = 0x4
	lnkHasRelPath    = 0x8
	lnkHasWorkDir    = 0x10
	lnkHasArgs       = 0x20
	lnkHasIcon       = 0x40
	lnkIsUnicode     = 0x80
	lnkForceNoInfo   = 0x100
	lnkHasExpString  = 0x200
	lnkHasDarwinID   = 0x1000
	sigEnvBlock      = 0xA0000001
	sigDarwinBlock   = 0xA0000006
	lnkHeaderSize    = 0x4C
	maxShortcutBytes = 1 << 20
)

var lnkCLSID = []byte{0x01, 0x14, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}

// Shortcut is what Jigglypuff needs to know about a .lnk file.
type Shortcut struct {
	Target  string
	Args    string
	Known   bool // false: target can't be resolved (MSI advertised, shell item, URL)
	Network bool
}

// ReadShortcut parses a .lnk file.
func ReadShortcut(path string) Shortcut {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() > maxShortcutBytes || fi.Size() < lnkHeaderSize {
		return Shortcut{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Shortcut{}
	}
	s := parseLnk(b)
	if s.Known && s.Target != "" && !filepath.IsAbs(s.Target) && !strings.HasPrefix(s.Target, `\\`) {
		s.Target = filepath.Join(filepath.Dir(path), s.Target)
	}
	return s
}

func le32(b []byte, off int) (uint32, bool) {
	if off < 0 || off+4 > len(b) {
		return 0, false
	}
	return binary.LittleEndian.Uint32(b[off:]), true
}

func le16(b []byte, off int) (uint16, bool) {
	if off < 0 || off+2 > len(b) {
		return 0, false
	}
	return binary.LittleEndian.Uint16(b[off:]), true
}

func cString(b []byte, off int) string {
	if off < 0 || off >= len(b) {
		return ""
	}
	end := bytes.IndexByte(b[off:], 0)
	if end < 0 {
		end = len(b) - off
	}
	// ANSI paths: decode as Latin-1, good enough for the ASCII paths used here.
	r := make([]rune, 0, end)
	for _, c := range b[off : off+end] {
		r = append(r, rune(c))
	}
	return string(r)
}

func wString(b []byte, off int) string {
	if off < 0 || off >= len(b) {
		return ""
	}
	var u []uint16
	for i := off; i+1 < len(b); i += 2 {
		c := binary.LittleEndian.Uint16(b[i:])
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}

func parseLnk(b []byte) Shortcut {
	var s Shortcut
	if len(b) < lnkHeaderSize {
		return s
	}
	if hs, _ := le32(b, 0); hs != lnkHeaderSize || !bytes.Equal(b[4:20], lnkCLSID) {
		return s
	}
	flags, _ := le32(b, 20)
	if flags&lnkHasDarwinID != 0 {
		return s // advertised (MSI) shortcut: Windows resolves it, we can't
	}
	off := lnkHeaderSize
	if flags&lnkHasIDList != 0 {
		n, ok := le16(b, off)
		if !ok {
			return s
		}
		off += 2 + int(n)
	}
	if flags&lnkHasLinkInfo != 0 && flags&lnkForceNoInfo == 0 {
		li := off
		size, ok1 := le32(b, li)
		hdr, ok2 := le32(b, li+4)
		liFlags, ok3 := le32(b, li+8)
		if !ok1 || !ok2 || !ok3 || li+int(size) > len(b) {
			return s
		}
		info := b[li : li+int(size)]
		if liFlags&0x1 != 0 { // VolumeIDAndLocalBasePath
			var base, suffix string
			if hdr >= 0x24 {
				bu, _ := le32(info, 28)
				su, _ := le32(info, 32)
				base, suffix = wString(info, int(bu)), wString(info, int(su))
			}
			if base == "" {
				bo, _ := le32(info, 16)
				so, _ := le32(info, 24)
				base, suffix = cString(info, int(bo)), cString(info, int(so))
			}
			if base != "" {
				s.Target = base + suffix
				s.Known = true
			}
		} else if liFlags&0x2 != 0 {
			s.Network = true
		}
		off += int(size)
	}
	readStr := func() string {
		n, ok := le16(b, off)
		if !ok {
			off = len(b)
			return ""
		}
		off += 2
		if flags&lnkIsUnicode != 0 {
			end := off + int(n)*2
			if end > len(b) {
				off = len(b)
				return ""
			}
			u := make([]uint16, n)
			for i := range u {
				u[i] = binary.LittleEndian.Uint16(b[off+i*2:])
			}
			off = end
			return string(utf16.Decode(u))
		}
		end := off + int(n)
		if end > len(b) {
			off = len(b)
			return ""
		}
		str := string(b[off:end])
		off = end
		return str
	}
	var relPath string
	if flags&lnkHasName != 0 {
		readStr()
	}
	if flags&lnkHasRelPath != 0 {
		relPath = readStr()
	}
	if flags&lnkHasWorkDir != 0 {
		readStr()
	}
	if flags&lnkHasArgs != 0 {
		s.Args = readStr()
	}
	if flags&lnkHasIcon != 0 {
		readStr()
	}
	// Extra data blocks.
	for off+8 <= len(b) {
		size, _ := le32(b, off)
		if size < 8 || off+int(size) > len(b) {
			break
		}
		sig, _ := le32(b, off+4)
		switch sig {
		case sigDarwinBlock:
			return Shortcut{}
		case sigEnvBlock:
			if flags&lnkHasExpString != 0 && size >= 0x314 {
				t := wString(b[off:off+int(size)], 268)
				if t == "" {
					t = cString(b[off:off+int(size)], 8)
				}
				if t != "" {
					s.Target = platform.ExpandEnv(t)
					s.Known = true
				}
			}
		}
		off += int(size)
	}
	if !s.Known && relPath != "" && !s.Network {
		s.Target = relPath
		s.Known = true
	}
	if s.Known {
		s.Target = platform.ExpandEnv(s.Target)
		if strings.HasPrefix(s.Target, `\\`) {
			s.Network = true
		}
	}
	return s
}
