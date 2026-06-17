package main

import (
	"os"

	"github.com/lobo235/claude-to-codex/internal/bridge"
)

var version = "dev"

func main() {
	bridge.Version = version
	bridge.Main(os.Args)
}
