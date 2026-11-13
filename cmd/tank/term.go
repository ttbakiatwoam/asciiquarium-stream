package main

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/render"
)

// termInput puts the terminal in raw mode and reports key presses and
// resizes, for -output term.
type termInput struct {
	out     *render.Terminal
	keys    chan byte
	resized chan os.Signal
	saved   *unix.Termios
}

func newTermInput() (*termInput, error) {
	fd := int(os.Stdin.Fd())
	saved, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	raw := *saved
	raw.Lflag &^= unix.ECHO | unix.ICANON
	raw.Cc[unix.VMIN], raw.Cc[unix.VTIME] = 1, 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		return nil, err
	}
	t := &termInput{
		out:     render.NewTerminal(os.Stdout),
		keys:    make(chan byte, 8),
		resized: make(chan os.Signal, 1),
		saved:   saved,
	}
	signal.Notify(t.resized, syscall.SIGWINCH)
	go func() {
		b := make([]byte, 1)
		for {
			if n, err := os.Stdin.Read(b); err != nil || n == 0 {
				return
			}
			t.keys <- b[0] | 0x20 // lower case
		}
	}()
	return t, nil
}

func (t *termInput) size() (int, int) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 {
		return 80, 24
	}
	return int(ws.Col), int(ws.Row)
}

func (t *termInput) restore() {
	t.out.Close()
	unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETS, t.saved)
}
