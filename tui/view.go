// Package tui is for rendering the TUI (Text User Interface) components
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Style for selected/highlighted items
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

	// Style for title/header
	titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)

	// Style for key hints (e.g., "Enter", "Backspace", "↑/↓")
	keyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	// Style for action hints (e.g., "type to search")
	actionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)

	// Style for search query
	searchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)

	// Style for success messages
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)

	// Style for error messages
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	// Style for dim/secondary text
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	// Style for labels in confirmation
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))

	// Style for loading/fetching messages (pastel blue/cyan)
	loadingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Italic(true)
)

const maxVisibleSubscriptions = 15

// Helper function to format instruction hints
func formatHint(keys string, action string) string {
	return keyStyle.Render(keys) + " " + action
}

func (m *Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Azure Policy Exemption CLI") + "\n\n")

	switch m.Step {
	case StepLoadingSubscriptions:
		b.WriteString(loadingStyle.Render("Retrieving subscriptions via Azure CLI...") + "\n")

	case StepSelectSubscription:
		b.WriteString("Select the subscription for the exemption:\n\n")
		start, end := visibleRange(m.Cursor, len(m.Subscriptions), maxVisibleSubscriptions)
		for i := start; i < end; i++ {
			sub := m.Subscriptions[i]
			cursor := " "
			if i == m.Cursor {
				cursor = ">"
			}
			marker := " "
			if i == m.SelectedSubscription {
				marker = "x"
			}
			line := fmt.Sprintf("%s [%s] %s (%s)", cursor, marker, sub.Name, sub.ShortID())
			if i == m.Cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.Subscriptions))) + "\n")
		if m.SubscriptionSearch != "" {
			b.WriteString("Search: " + searchStyle.Render(m.SubscriptionSearch) + "\n")
			b.WriteString(formatHint("Type", "to search") + ", " + formatHint("Esc", "to clear") + ", " + formatHint("Enter", "to select") + "\n")
		} else {
			b.WriteString(formatHint("↑/↓", "move") + ", " + actionStyle.Render("type to search") + ", " + formatHint("Enter", "select") + "\n")
		}

	case StepLoadingAssignments:
		b.WriteString(loadingStyle.Render("Loading policy assignments for the selected subscription...") + "\n")

	case StepSelectAssignment:
		sub := m.CurrentSubscription()
		fmt.Fprintf(&b, "Policy assignments for subscription %s (%s):\n\n", sub.Name, sub.ShortID())
		start, end := visibleRange(m.Cursor, len(m.Assignments), maxVisibleSubscriptions)
		for i := start; i < end; i++ {
			assign := m.Assignments[i]
			isBlocked := m.IsDefinitionBlocked(assign.PolicyDefinitionID)
			cursor := " "
			if i == m.Cursor {
				cursor = ">"
			}
			marker := " "
			if isBlocked {
				marker = "-" // Blocked indicator
			} else if i == m.SelectedAssignment {
				marker = "x"
			}
			var line string
			if isBlocked {
				line = fmt.Sprintf("%s [%s] %s (%s) [blocked]", cursor, marker, assign.DisplayLabel(), assign.ShortID())
				line = dimStyle.Render(line)
			} else {
				line = fmt.Sprintf("%s [%s] %s (%s)", cursor, marker, assign.DisplayLabel(), assign.ShortID())
				if i == m.Cursor {
					line = selectedStyle.Render(line)
				}
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.Assignments))) + "\n")
		if m.AssignmentSearch != "" {
			b.WriteString("Search: " + searchStyle.Render(m.AssignmentSearch) + "\n")
			b.WriteString(formatHint("Type", "to search") + ", " + formatHint("Esc", "to clear") + ", " + formatHint("Enter", "select") + ", " + formatHint("Backspace", "delete") + "\n")
		} else {
			b.WriteString(formatHint("↑/↓", "move") + ", " + actionStyle.Render("type to search") + ", " + formatHint("Enter", "select") + ", " + formatHint("Backspace", "go back") + "\n")
		}

	case StepLoadingAssignmentDefinitions:
		b.WriteString(loadingStyle.Render("Loading assignment details...") + "\n")

	case StepAssignmentScope:
		b.WriteString("This assignment contains multiple policy definitions.\n\n")
		options := []string{"Exempt entire assignment", "Exempt specific definitions"}
		for i, opt := range options {
			cursor := " "
			if i == m.Cursor {
				cursor = ">"
			}
			line := fmt.Sprintf("%s %s", cursor, opt)
			if i == m.Cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		b.WriteString("\n" + formatHint("↑/↓", "move") + ", " + formatHint("Enter", "choose") + ", " + formatHint("Backspace", "go back") + "\n")

	case StepSelectDefinitions:
		b.WriteString("Select the policy definitions to exempt:\n\n")
		start, end := visibleRange(m.Cursor, len(m.AssignmentDefinitions), maxVisibleSubscriptions)
		for i := start; i < end; i++ {
			ref := m.AssignmentDefinitions[i]
			isBlocked := m.IsDefinitionBlocked(ref.PolicyDefinitionID)
			cursor := " "
			if i == m.Cursor {
				cursor = ">"
			}
			marker := " "
			if isBlocked {
				marker = "-" // Blocked indicator
			} else if m.SelectedDefinitionIDs[ref.ReferenceID] {
				marker = "x"
			}
			var line string
			if isBlocked {
				line = fmt.Sprintf("%s [%s] %s (%s) [blocked]", cursor, marker, ref.DisplayName, ref.ReferenceID)
				line = dimStyle.Render(line)
			} else {
				line = fmt.Sprintf("%s [%s] %s (%s)", cursor, marker, ref.DisplayName, ref.ReferenceID)
				if i == m.Cursor {
					line = selectedStyle.Render(line)
				}
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.AssignmentDefinitions))) + "\n")
		if m.DefinitionSearch != "" {
			b.WriteString("Search: " + searchStyle.Render(m.DefinitionSearch) + "\n")
			b.WriteString(formatHint("Type", "to search") + ", " + formatHint("Esc", "to clear") + ", " + formatHint("Space", "toggle") + ", " + formatHint("Enter", "continue") + "\n")
		} else {
			b.WriteString(formatHint("↑/↓", "move") + ", " + actionStyle.Render("type to search") + ", " + formatHint("Space", "toggle") + ", " + formatHint("Enter", "continue") + ", " + formatHint("Backspace", "go back") + "\n")
		}

	case StepLoadingResourceGroups:
		b.WriteString(loadingStyle.Render("Loading resource groups...") + "\n")

	case StepSelectResourceGroup:
		b.WriteString("Select the scope for the exemption:\n\n")
		start, end := visibleRange(m.Cursor, len(m.ResourceGroups), maxVisibleSubscriptions)
		for i := start; i < end; i++ {
			rg := m.ResourceGroups[i]
			cursor := " "
			if i == m.Cursor {
				cursor = ">"
			}
			marker := " "
			if i == m.SelectedResourceGroup {
				marker = "x"
			}
			line := fmt.Sprintf("%s [%s] %s", cursor, marker, rg.Name)
			if i == m.Cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.ResourceGroups))) + "\n")
		b.WriteString(formatHint("↑/↓", "move") + ", " + formatHint("Enter", "select") + ", " + formatHint("Backspace", "go back") + "\n")

	case StepTicket:
		assign := m.CurrentAssignment()
		fmt.Fprintf(&b, labelStyle.Render("Assignment: ")+"%s\n\n", assign.DisplayLabel())
		if m.PartialExemption && len(m.SelectedDefinitionIDs) > 0 {
			b.WriteString(labelStyle.Render("Definitions selected:") + "\n")
			for _, ref := range m.AssignmentDefinitions {
				if m.SelectedDefinitionIDs[ref.ReferenceID] {
					fmt.Fprintf(&b, "  • %s\n", ref.DisplayName)
				}
			}
			b.WriteString("\n")
		}
		b.WriteString("Provide the tracking ticket number linked to this exemption:\n\n")
		b.WriteString(m.TicketInput.View() + "\n")
		b.WriteString("\n" + formatHint("Backspace", "on empty input to go back") + "\n")

	case StepUsers:
		assign := m.CurrentAssignment()
		b.WriteString(labelStyle.Render("Ticket: ") + m.Ticket + "\n")
		b.WriteString(labelStyle.Render("Assignment: ") + assign.DisplayLabel() + "\n\n")
		b.WriteString("Who is requesting this exemption? (comma separated)\n\n")
		b.WriteString(m.UserInput.View() + "\n")
		b.WriteString("\n" + formatHint("Backspace", "on empty input to go back") + "\n")

	case StepExpirationChoice:
		b.WriteString("Do you want to set an expiration date?\n\n")
		options := []string{"Unlimited (No expiration)", "Set expiration date"}
		for i, opt := range options {
			cursor := " "
			if i == m.Cursor {
				cursor = ">"
			}
			line := fmt.Sprintf("%s %s", cursor, opt)
			if i == m.Cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		b.WriteString("\n" + formatHint("↑/↓", "move") + ", " + formatHint("Enter", "choose") + ", " + formatHint("Backspace", "go back") + "\n")

	case StepExpirationDate:
		b.WriteString("Enter the expiration date (YYYY-MM-DD):\n\n")
		b.WriteString(m.ExpirationInput.View() + "\n")
		b.WriteString("\n" + formatHint("Backspace", "on empty input to go back") + "\n")

	case StepConfirm:
		b.WriteString(titleStyle.Render("Review Exemption Details") + "\n\n")
		sub := m.CurrentSubscription()
		assign := m.CurrentAssignment()
		rg := m.ResourceGroups[m.SelectedResourceGroup]
		b.WriteString(labelStyle.Render("Subscription: ") + fmt.Sprintf("%s (%s)\n", sub.Name, sub.ShortID()))
		b.WriteString(labelStyle.Render("Scope: ") + rg.Name + "\n")
		b.WriteString(labelStyle.Render("Assignment: ") + assign.DisplayLabel() + "\n")
		if m.PartialExemption && len(m.SelectedDefinitionIDs) > 0 {
			b.WriteString(labelStyle.Render("Definitions:") + "\n")
			for _, ref := range m.AssignmentDefinitions {
				if m.SelectedDefinitionIDs[ref.ReferenceID] {
					fmt.Fprintf(&b, "  • %s\n", ref.DisplayName)
				}
			}
		} else {
			b.WriteString(labelStyle.Render("Definitions: ") + "Entire assignment\n")
		}
		b.WriteString(labelStyle.Render("Ticket: ") + m.Ticket + "\n")
		b.WriteString(labelStyle.Render("Requesters: ") + m.RequestUser + "\n")
		if m.ExpirationDate != "" {
			b.WriteString(labelStyle.Render("Expires on: ") + m.ExpirationDate + "\n")
		} else {
			b.WriteString(labelStyle.Render("Expires on: ") + "Unlimited\n")
		}
		b.WriteString("\n" + formatHint("Enter", "create exemption") + ", " + formatHint("Backspace", "go back") + ", " + formatHint("q", "abort") + "\n")

	case StepCreating:
		b.WriteString(loadingStyle.Render("Creating policy exemption via Azure CLI...") + "\n")

	case StepDone:
		b.WriteString(successStyle.Render("Exemption created successfully!") + "\n\n")
		b.WriteString(dimStyle.Render("Azure CLI response:") + "\n\n")
		if m.CreateOutput == "" {
			b.WriteString(dimStyle.Render("No output returned.") + "\n")
		} else {
			b.WriteString(m.CreateOutput + "\n")
		}
		b.WriteString("\n" + formatHint("Enter", "create another exemption") + ", " + formatHint("q", "exit") + "\n")

	case StepError:
		b.WriteString(errorStyle.Render("Error: ") + fmt.Sprintf("%v\n\n", m.Err))
		b.WriteString(formatHint("q", "exit") + "\n")
	}

	if m.Status != "" {
		b.WriteString("\n" + errorStyle.Render(m.Status) + "\n")
	}

	// Add global quit hint for steps that don't already show it
	if m.Step != StepDone && m.Step != StepError && m.Step != StepConfirm {
		b.WriteString("\n" + dimStyle.Render("Press "+keyStyle.Render("q")+" to quit at any time.") + "\n")
	}

	return b.String()
}

func visibleRange(cursor, total, limit int) (start, end int) {
	if limit <= 0 || total <= limit {
		return 0, total
	}
	start = cursor - limit/2
	if start < 0 {
		start = 0
	}
	end = start + limit
	if end > total {
		end = total
		start = end - limit
	}
	return start, end
}
