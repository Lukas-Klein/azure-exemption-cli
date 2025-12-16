// Package tui is for rendering the TUI (Text User Interface) components
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

const maxVisibleSubscriptions = 15

func (m *Model) View() string {
	var b strings.Builder
	b.WriteString("Azure Policy Exemption CLI\n\n")

	switch m.Step {
	case StepLoadingSubscriptions:
		b.WriteString("Retrieving subscriptions via Azure CLI...\n")

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
		fmt.Fprintf(&b, "\nShowing %d-%d of %d\n", start+1, end, len(m.Subscriptions))
		b.WriteString("↑/↓ to move, Enter to select.\n")

	case StepLoadingAssignments:
		b.WriteString("Loading policy assignments for the selected subscription...\n")

	case StepSelectAssignment:
		sub := m.CurrentSubscription()
		fmt.Fprintf(&b, "Policy assignments for subscription %s (%s):\n\n", sub.Name, sub.ShortID())
		start, end := visibleRange(m.Cursor, len(m.Assignments), maxVisibleSubscriptions)
		for i := start; i < end; i++ {
			assign := m.Assignments[i]
			cursor := " "
			if i == m.Cursor {
				cursor = ">"
			}
			marker := " "
			if i == m.SelectedAssignment {
				marker = "x"
			}
			line := fmt.Sprintf("%s [%s] %s (%s)", cursor, marker, assign.DisplayLabel(), assign.ShortID())
			if i == m.Cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		fmt.Fprintf(&b, "\nShowing %d-%d of %d\n", start+1, end, len(m.Assignments))
		b.WriteString("↑/↓ to move, Enter to select.\n")

	case StepLoadingAssignmentDefinitions:
		b.WriteString("Loading assignment details...\n")

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
		b.WriteString("\n↑/↓ to move, Enter to choose.\n")

	case StepSelectDefinitions:
		b.WriteString("Select the policy definitions to exempt:\n\n")
		start, end := visibleRange(m.Cursor, len(m.AssignmentDefinitions), maxVisibleSubscriptions)
		for i := start; i < end; i++ {
			ref := m.AssignmentDefinitions[i]
			cursor := " "
			if i == m.Cursor {
				cursor = ">"
			}
			marker := " "
			if m.SelectedDefinitionIDs[ref.ReferenceID] {
				marker = "x"
			}
			line := fmt.Sprintf("%s [%s] %s (%s)", cursor, marker, ref.DisplayName, ref.ReferenceID)
			if i == m.Cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		fmt.Fprintf(&b, "\nShowing %d-%d of %d\n", start+1, end, len(m.AssignmentDefinitions))
		b.WriteString("↑/↓ to move, Space to toggle, Enter to continue.\n")

	case StepLoadingResourceGroups:
		b.WriteString("Loading resource groups...\n")

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
		fmt.Fprintf(&b, "\nShowing %d-%d of %d\n", start+1, end, len(m.ResourceGroups))
		b.WriteString("↑/↓ to move, Enter to select.\n")

	case StepTicket:
		assign := m.CurrentAssignment()
		fmt.Fprintf(&b, "Assignment selected: %s\n\n", assign.DisplayLabel())
		if m.PartialExemption && len(m.SelectedDefinitionIDs) > 0 {
			b.WriteString("Definitions selected:\n")
			for _, ref := range m.AssignmentDefinitions {
				if m.SelectedDefinitionIDs[ref.ReferenceID] {
					fmt.Fprintf(&b, "• %s (%s)\n", ref.DisplayName, ref.ReferenceID)
				}
			}
			b.WriteString("\n")
		}
		b.WriteString("Provide the tracking ticket number linked to this exemption:\n\n")
		b.WriteString(m.TicketInput.View() + "\n")

	case StepUsers:
		assign := m.CurrentAssignment()
		fmt.Fprintf(&b, "Ticket: %s\nAssignment: %s\n\n", m.Ticket, assign.DisplayLabel())
		b.WriteString("Who is requesting this exemption? (comma separated)\n\n")
		b.WriteString(m.UserInput.View() + "\n")

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
		b.WriteString("\n↑/↓ to move, Enter to choose.\n")

	case StepExpirationDate:
		b.WriteString("Enter the expiration date (YYYY-MM-DD):\n\n")
		b.WriteString(m.ExpirationInput.View() + "\n")

	case StepConfirm:
		sub := m.CurrentSubscription()
		assign := m.CurrentAssignment()
		rg := m.ResourceGroups[m.SelectedResourceGroup]
		fmt.Fprintf(&b, "Subscription: %s (%s)\n", sub.Name, sub.ShortID())
		fmt.Fprintf(&b, "Scope: %s\n", rg.Name)
		fmt.Fprintf(&b, "Assignment: %s\n", assign.DisplayLabel())
		if m.PartialExemption && len(m.SelectedDefinitionIDs) > 0 {
			b.WriteString("Definitions:\n")
			for _, ref := range m.AssignmentDefinitions {
				if m.SelectedDefinitionIDs[ref.ReferenceID] {
					fmt.Fprintf(&b, "  %s (%s)\n", ref.DisplayName, ref.ReferenceID)
				}
			}
		} else {
			b.WriteString("Definitions: Entire assignment\n")
		}
		fmt.Fprintf(&b, "Ticket: %s\n", m.Ticket)
		fmt.Fprintf(&b, "Requesters: %s\n", m.RequestUser)
		if m.ExpirationDate != "" {
			fmt.Fprintf(&b, "Expires on: %s\n", m.ExpirationDate)
		} else {
			b.WriteString("Expires on: Unlimited\n")
		}
		b.WriteString("\nPress Enter to create the exemption or q to abort.\n")

	case StepCreating:
		b.WriteString("Creating policy exemption via Azure CLI...\n")

	case StepDone:
		b.WriteString("Azure CLI response:\n\n")
		if m.CreateOutput == "" {
			b.WriteString("No output returned.\n")
		} else {
			b.WriteString(m.CreateOutput + "\n")
		}
		b.WriteString("\nPress q to exit.\n")

	case StepError:
		fmt.Fprintf(&b, "Error: %v\n\nPress q to exit.\n", m.Err)
	}

	if m.Status != "" {
		b.WriteString("\n" + m.Status + "\n")
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
