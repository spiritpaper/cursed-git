// cmd/cg/main.go
package main

import (
	"os"

	"github.com/spiritpaper/cursed-git/internal/cli"
)

func main() {
	// all the logic lives in internal/cli so it can be tested without building the binary
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
