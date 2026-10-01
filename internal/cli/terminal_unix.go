//go:build unix

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// widthOf reads the terminal size behind f with the standard TIOCGWINSZ ioctl.
//
// A winsize is four uint16 fields and only the column count is wanted. A file
// or a pipe has no size, and the kernel fails the call, which reads here as 0 —
// "unknown", not a zero-width terminal.
func widthOf(f *os.File) int {
	var ws struct {
		row, col, xpixel, ypixel uint16
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 || ws.col == 0 {
		return 0
	}
	return int(ws.col)
}
