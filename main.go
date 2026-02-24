// chatlog is a WeChat chat log management tool for macOS.
// It extracts encryption keys from WeChat memory, decrypts SQLite databases,
// serves HTTP API and MCP protocol, auto-decrypts on file changes, pushes
// webhooks, and batch-decrypts .dat image files. macOS + WeChat v4 only.
package main

import (
	"log"

	"github.com/TE0dollary/chatlog-bot/cmd/chatlog"
)

// main is the program entry point, dispatching via cobra CLI subcommands.
// Without a subcommand it launches the TUI. Available subcommands:
//   - chatlog key            : extract WeChat encryption key
//   - chatlog decrypt        : decrypt database files
//   - chatlog server         : start HTTP server without TUI
//   - chatlog batch-decrypt  : batch decrypt .dat images
//   - chatlog dumpmemory     : dump WeChat process memory (debug)
//   - chatlog version        : show version info
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	chatlog.Execute()
}
