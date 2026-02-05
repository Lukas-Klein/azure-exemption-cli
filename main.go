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

func main() {
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
