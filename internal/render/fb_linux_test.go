//go:build linux

package render

import (
	"testing"
	"unsafe"
)

// The structs must match the kernel's struct fb_var_screeninfo and
// struct fb_fix_screeninfo byte for byte.
func TestFramebufferStructSizes(t *testing.T) {
	if got := unsafe.Sizeof(fbVarScreenInfo{}); got != 160 {
		t.Errorf("fb_var_screeninfo is %d bytes, want 160", got)
	}
	want := uintptr(68)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		want = 80
	}
	if got := unsafe.Sizeof(fbFixScreenInfo{}); got != want {
		t.Errorf("fb_fix_screeninfo is %d bytes, want %d", got, want)
	}
	if off := unsafe.Offsetof(fbFixScreenInfo{}.LineLength); off != 48 && unsafe.Sizeof(uintptr(0)) == 8 {
		t.Errorf("line_length at offset %d, want 48", off)
	}
}
