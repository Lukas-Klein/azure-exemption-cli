package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Lukas-Klein/azure-exemption-cli/azure"
	"github.com/Lukas-Klein/azure-exemption-cli/config"
	"github.com/Lukas-Klein/azure-exemption-cli/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// Set via ldflags at build time by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("azure-exemption-cli %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	ctx := context.Background()
	client := azure.NewClient()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := client.EnsureLogin(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Azure login failed: %v\n", err)
		os.Exit(1)
	}

	blockedDefs := cfg.BlockedDefinitionsMap()
	p := tea.NewProgram(tui.NewModel(ctx, client, blockedDefs))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
