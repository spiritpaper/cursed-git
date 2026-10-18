//go:build windows

// internal/cli/isatty_windows.go
package cli

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const enableVirtualTerminalProcessing = 0x0004

// isTerminal asks the console: only a real console answers GetConsoleMode.
// (Pipes, files and NUL don't.) Git Bash's mintty is a pipe to Windows, so there she stays
// quiet; Windows Terminal, PowerShell and cmd are consoles.
func isTerminal(f *os.File) bool {
	var mode uint32
	ok, _, _ := procGetConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&mode)))
	return ok != 0
}

// her face and the screen-clearing tricks use ANSI escapes: turn them on for this console
func init() {
	var mode uint32
	h := os.Stderr.Fd()
	if ok, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode))); ok != 0 {
		procSetConsoleMode.Call(h, uintptr(mode|enableVirtualTerminalProcessing))
	}
}
