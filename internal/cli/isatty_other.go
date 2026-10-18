//go:build !linux && !darwin && !windows

// internal/cli/isatty_other.go
package cli

import "os"

// On other systems (the BSDs, ...) she stays quiet for now: better no curses than curses in scripts.
func isTerminal(f *os.File) bool { return false }
