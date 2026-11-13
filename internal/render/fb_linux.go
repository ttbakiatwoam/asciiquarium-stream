//go:build linux

package render

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Linux framebuffer ioctls and structs (linux/fb.h). The struct layouts use
// uintptr where the kernel uses unsigned long, so they match on both 32-bit
// and 64-bit Raspberry Pi OS.
const (
	fbioGetVScreenInfo = 0x4600
	fbioGetFScreenInfo = 0x4602
	kdSetMode          = 0x4B3A
	kdText             = 0x00
	kdGraphics         = 0x01
)

type fbBitfield struct{ Offset, Length, MsbRight uint32 }

type fbVarScreenInfo struct {
	XRes, YRes, XResVirtual, YResVirtual, XOffset, YOffset uint32
	BitsPerPixel, Grayscale                                uint32
	Red, Green, Blue, Transp                               fbBitfield
	NonStd, Activate, Height, Width, AccelFlags            uint32
	PixClock, LeftMargin, RightMargin, UpperMargin         uint32
	LowerMargin, HsyncLen, VsyncLen, Sync, Vmode, Rotate   uint32
	Colorspace                                             uint32
	Reserved                                               [4]uint32
}

type fbFixScreenInfo struct {
	ID                             [16]byte
	SmemStart                      uintptr
	SmemLen, Type, TypeAux, Visual uint32
	XPanStep, YPanStep, YWrapStep  uint16
	LineLength                     uint32
	MmioStart                      uintptr
	MmioLen, Accel                 uint32
	Capabilities                   uint16
	Reserved                       [2]uint16
}

// Framebuffer draws to a Linux framebuffer device such as /dev/fb0, which
// on a Raspberry Pi is the HDMI output.
type Framebuffer struct {
	*Canvas
	f   *os.File
	mem []byte
	tty *os.File
}

// OpenFramebuffer maps the framebuffer device. If tty is not empty (e.g.
// /dev/tty1) that console is switched to graphics mode so the blinking
// cursor and console text don't draw over the aquarium.
func OpenFramebuffer(dev, tty string, cellH int) (*Framebuffer, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	var vi fbVarScreenInfo
	var fi fbFixScreenInfo
	if err := ioctl(f, fbioGetVScreenInfo, unsafe.Pointer(&vi)); err != nil {
		f.Close()
		return nil, fmt.Errorf("FBIOGET_VSCREENINFO: %w", err)
	}
	if err := ioctl(f, fbioGetFScreenInfo, unsafe.Pointer(&fi)); err != nil {
		f.Close()
		return nil, fmt.Errorf("FBIOGET_FSCREENINFO: %w", err)
	}
	var pf PixelFormat
	switch vi.BitsPerPixel {
	case 16, 24, 32:
		pf = PixelFormat{
			BytesPerPixel: int(vi.BitsPerPixel / 8),
			ROff:          vi.Red.Offset, RLen: vi.Red.Length,
			GOff: vi.Green.Offset, GLen: vi.Green.Length,
			BOff: vi.Blue.Offset, BLen: vi.Blue.Length,
		}
	default:
		f.Close()
		return nil, fmt.Errorf("unsupported framebuffer depth %d bpp", vi.BitsPerPixel)
	}
	stride := int(fi.LineLength)
	if stride == 0 {
		stride = int(vi.XResVirtual) * pf.BytesPerPixel
	}
	size := stride * int(vi.YResVirtual)
	mem, err := unix.Mmap(int(f.Fd()), 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("mmap %s: %w", dev, err)
	}
	if cellH <= 0 {
		cellH = int(vi.YRes) / 54
	}
	font, err := NewFont(cellH)
	if err != nil {
		unix.Munmap(mem)
		f.Close()
		return nil, err
	}
	// draw into the visible page
	visible := mem[int(vi.YOffset)*stride:]
	fb := &Framebuffer{
		Canvas: NewCanvas(int(vi.XRes), int(vi.YRes), stride, visible, pf, font),
		f:      f,
		mem:    mem,
	}
	if tty != "" {
		if t, err := os.OpenFile(tty, os.O_RDWR, 0); err == nil {
			if err := unix.IoctlSetInt(int(t.Fd()), kdSetMode, kdGraphics); err == nil {
				fb.tty = t
			} else {
				t.Close()
			}
		}
	}
	return fb, nil
}

// Close restores the console and unmaps the framebuffer.
func (fb *Framebuffer) Close() error {
	fb.Canvas.Clear()
	if fb.tty != nil {
		unix.IoctlSetInt(int(fb.tty.Fd()), kdSetMode, kdText)
		fb.tty.Close()
	}
	unix.Munmap(fb.mem)
	return fb.f.Close()
}

func ioctl(f *os.File, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}
