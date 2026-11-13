package music

import (
	"os"
	"path/filepath"
	"testing"
)

func frame(id string, text []byte, v4 bool) []byte {
	n := len(text)
	size := []byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	if v4 {
		size = []byte{byte(n>>21) & 0x7f, byte(n>>14) & 0x7f, byte(n>>7) & 0x7f, byte(n) & 0x7f}
	}
	return append(append(append([]byte(id), size...), 0, 0), text...)
}

func id3(ver byte, frames ...[]byte) []byte {
	var body []byte
	for _, f := range frames {
		body = append(body, f...)
	}
	n := len(body)
	hdr := []byte{'I', 'D', '3', ver, 0, 0, byte(n>>21) & 0x7f, byte(n>>14) & 0x7f, byte(n>>7) & 0x7f, byte(n) & 0x7f}
	return append(append(hdr, body...), make([]byte, 64)...)
}

func write(t *testing.T, name string, b []byte) string {
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadTags(t *testing.T) {
	utf16 := []byte{1, 0xff, 0xfe, 'S', 0, 'e', 0, 'a', 0}
	cases := []struct {
		name string
		data []byte
		want Tags
	}{
		{"v23.mp3", id3(3, frame("TIT2", []byte("\x00Deep Blue"), false), frame("TPE1", []byte("\x03asciifish"), false)), Tags{"Deep Blue", "asciifish"}},
		{"v24.mp3", id3(4, frame("TIT2", []byte("\x03Kelp Forest\x00"), true)), Tags{Title: "Kelp Forest"}},
		{"utf16.mp3", id3(3, frame("TIT2", utf16, false)), Tags{Title: "Sea"}},
		{"Tide Pool.mp3", []byte("no tags here"), Tags{Title: "Tide Pool"}},
	}
	for _, c := range cases {
		if got := ReadTags(write(t, c.name, c.data)); got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
	v1 := make([]byte, 200)
	copy(v1[72:], "TAG")
	copy(v1[75:], "Old Song")
	copy(v1[105:], "Old Artist")
	if got := ReadTags(write(t, "v1.mp3", v1)); got != (Tags{"Old Song", "Old Artist"}) {
		t.Errorf("v1: got %+v", got)
	}
}
