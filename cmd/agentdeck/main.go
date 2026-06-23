// Command agentdeck is the AgentDeck CLI and dashboard server entrypoint.
package main

import (
	"os"

	"github.com/agentdeck/agentdeck/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
