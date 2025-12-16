package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Lukas-Klein/azure-exemption-cli/internal/azure"
	"github.com/Lukas-Klein/azure-exemption-cli/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	ctx := context.Background()
	client := azure.NewClient()

	if err := client.EnsureLogin(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Azure login failed: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(tui.NewModel(ctx, client))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
