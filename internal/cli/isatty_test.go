package cli

import (
	"os"
	"testing"
)

// /dev/null is a character device, which fooled the first version of isTerminal.
func TestDevNullIsNotATerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("/dev/null was treated as a terminal")
	}
}
