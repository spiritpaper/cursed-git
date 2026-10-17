//go:build linux

// internal/cli/isatty_linux.go
package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminal asks the terminal driver: only a real terminal answers TCGETS.
// (Checking for a "character device" isn't enough: /dev/null is one too.)
func isTerminal(f *os.File) bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&t)))
	return errno == 0
}
