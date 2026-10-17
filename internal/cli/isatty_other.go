//go:build !linux && !darwin

// internal/cli/isatty_other.go
package cli

import "os"

// On other systems (Windows) she stays quiet for now: better no curses than curses in scripts.
func isTerminal(f *os.File) bool { return false }
