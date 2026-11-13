// Package music plays a folder of MP3s through an external player and keeps
// track of what is playing, so the tank can show the song title.
package music

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// Tags are the parts of an MP3's metadata we show.
type Tags struct {
	Title  string
	Artist string
}

// ReadTags reads ID3v2 (2.2, 2.3, 2.4) or ID3v1 tags from an MP3 file.
// Missing titles fall back to the file name without its extension.
func ReadTags(path string) Tags {
	t := Tags{}
	if f, err := os.Open(path); err == nil {
		t = readID3v2(f)
		if t.Title == "" {
			t = readID3v1(f)
		}
		f.Close()
	}
	if t.Title == "" {
		base := filepath.Base(path)
		t.Title = strings.TrimSuffix(base, filepath.Ext(base))
	}
	return t
}

func syncsafe(b []byte) int {
	return int(b[0]&0x7f)<<21 | int(b[1]&0x7f)<<14 | int(b[2]&0x7f)<<7 | int(b[3]&0x7f)
}

func readID3v2(r io.Reader) Tags {
	var t Tags
	hdr := make([]byte, 10)
	if _, err := io.ReadFull(r, hdr); err != nil || string(hdr[:3]) != "ID3" {
		return t
	}
	ver := hdr[3]
	size := syncsafe(hdr[6:10])
	if size > 4<<20 {
		size = 4 << 20 // don't read huge embedded artwork just for a title
	}
	body := make([]byte, size)
	n, _ := io.ReadFull(r, body)
	body = body[:n]
	if hdr[5]&0x40 != 0 && ver >= 3 && len(body) >= 4 { // extended header
		var ext int
		if ver == 4 {
			ext = syncsafe(body[:4])
		} else {
			ext = int(binary.BigEndian.Uint32(body[:4])) + 4
		}
		if ext < len(body) {
			body = body[ext:]
		}
	}
	idLen, hdrLen := 4, 10
	if ver == 2 {
		idLen, hdrLen = 3, 6
	}
	for len(body) >= hdrLen && body[0] != 0 {
		id := string(body[:idLen])
		var fsize int
		switch ver {
		case 2:
			fsize = int(body[3])<<16 | int(body[4])<<8 | int(body[5])
		case 3:
			fsize = int(binary.BigEndian.Uint32(body[4:8]))
		default:
			fsize = syncsafe(body[4:8])
		}
		if fsize < 0 || hdrLen+fsize > len(body) {
			break
		}
		data := body[hdrLen : hdrLen+fsize]
		switch id {
		case "TIT2", "TT2":
			t.Title = decodeText(data)
		case "TPE1", "TP1":
			t.Artist = decodeText(data)
		}
		body = body[hdrLen+fsize:]
	}
	return t
}

func decodeText(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	enc, b := b[0], b[1:]
	var s string
	switch enc {
	case 1, 2: // UTF-16 with BOM, UTF-16BE
		be := enc == 2
		if len(b) >= 2 {
			if b[0] == 0xff && b[1] == 0xfe {
				be, b = false, b[2:]
			} else if b[0] == 0xfe && b[1] == 0xff {
				be, b = true, b[2:]
			}
		}
		u := make([]uint16, 0, len(b)/2)
		for i := 0; i+1 < len(b); i += 2 {
			if be {
				u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
			} else {
				u = append(u, uint16(b[i+1])<<8|uint16(b[i]))
			}
		}
		s = string(utf16.Decode(u))
	case 3: // UTF-8
		s = string(b)
	default: // ISO-8859-1
		r := make([]rune, len(b))
		for i, c := range b {
			r[i] = rune(c)
		}
		s = string(r)
	}
	// multiple values are NUL separated; keep the first
	if i := strings.IndexRune(s, 0); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func readID3v1(f *os.File) Tags {
	var t Tags
	st, err := f.Stat()
	if err != nil || st.Size() < 128 {
		return t
	}
	b := make([]byte, 128)
	if _, err := f.ReadAt(b, st.Size()-128); err != nil || string(b[:3]) != "TAG" {
		return t
	}
	field := func(x []byte) string {
		if i := bytes.IndexByte(x, 0); i >= 0 {
			x = x[:i]
		}
		return strings.TrimSpace(decodeText(append([]byte{0}, x...)))
	}
	t.Title = field(b[3:33])
	t.Artist = field(b[33:63])
	return t
}
