// Command chuck is the Chuck CLI and dashboard server entrypoint.
package main

import (
	"os"

	"github.com/AsaphNoam/Chuck/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
