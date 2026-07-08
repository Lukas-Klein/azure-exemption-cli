package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Lukas-Klein/azure-exemption-cli/azure"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		return m, m.handleKey(msg)

	case subscriptionsLoadedMsg:
		if msg.err != nil {
			return m.Fail(msg.err)
		}
		if len(msg.subscriptions) == 0 {
			return m.Fail(errors.New("no subscriptions returned by Azure CLI"))
		}
		m.Subscriptions = msg.subscriptions
		m.Cursor = 0
		m.SelectedSubscription = -1
		m.Step = StepSelectSubscription
		m.Status = "" // Help text is in the view
		return m, nil

	case assignmentsLoadedMsg:
		if msg.err != nil {
			return m.Fail(msg.err)
		}
		if len(msg.assignments) == 0 {
			sub := m.CurrentSubscription()
			return m.Fail(fmt.Errorf("no policy assignments were returned for subscription %s (%s)", sub.Name, sub.ShortID()))
		}
		m.Assignments = msg.assignments
		m.SelectedAssignment = -1
		m.AssignmentDefinitions = nil
		m.SelectedDefinitionIDs = make(map[string]bool)
		m.PartialExemption = false
		m.AssignmentSearch = ""
		m.Cursor = 0
		m.Step = StepSelectAssignment
		m.Status = "" // Help text is in the view
		return m, nil

	case assignmentDefinitionsLoadedMsg:
		if msg.err != nil {
			return m.Fail(msg.err)
		}
		m.AssignmentDefinitions = msg.definitions
		m.SelectedDefinitionIDs = make(map[string]bool)
		m.PartialExemption = false
		if len(msg.definitions) > 1 {
			m.Step = StepAssignmentScope
			m.Cursor = 0
			m.Status = "" // Help text is in the view
		} else {
			m.PartialExemption = false
			m.Step = StepLoadingResourceGroups
			m.Status = "" // Loading state shown in view
			return m, fetchResourceGroupsCmd(m.ctx, m.azureClient, m.CurrentSubscription())
		}
		return m, nil

	case resourceGroupsLoadedMsg:
		if msg.err != nil {
			return m.Fail(msg.err)
		}
		// Prepend "Entire Subscription" option
		sub := m.CurrentSubscription()
		entireSub := azure.ResourceGroup{
			Name: "Entire Subscription",
			ID:   sub.Scope(),
		}
		m.ResourceGroups = append([]azure.ResourceGroup{entireSub}, msg.resourceGroups...)
		m.SelectedResourceGroup = -1
		m.Cursor = 0
		m.Step = StepSelectResourceGroup
		m.Status = "" // Help text is in the view
		return m, nil

	case exemptionCreatedMsg:
		if msg.err != nil {
			return m.Fail(msg.err)
		}
		m.CreateOutput = msg.output
		m.Step = StepDone
		m.Status = "" // Help text is in the view
		return m, nil
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch m.Step {
	case StepSelectSubscription:
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Subscriptions)-1 {
				m.Cursor++
			}
		case "enter":
			if len(m.Subscriptions) == 0 {
				return nil
			}
			m.SelectedSubscription = m.Cursor
			m.SubscriptionSearch = ""
			m.Step = StepLoadingAssignments
			m.Status = "" // Loading state shown in view
			return fetchAssignmentsCmd(m.ctx, m.azureClient, m.CurrentSubscription())
		case "backspace":
			// Delete the last character from the search buffer
			if len(m.SubscriptionSearch) > 0 {
				m.SubscriptionSearch = m.SubscriptionSearch[:len(m.SubscriptionSearch)-1]
				if m.SubscriptionSearch == "" {
					m.Status = "" // Help text is in the view
				} else {
					// Re-search with the updated query
					searchLower := strings.ToLower(m.SubscriptionSearch)
					for i, sub := range m.Subscriptions {
						if strings.Contains(strings.ToLower(sub.Name), searchLower) {
							m.Cursor = i
							break
						}
					}
				}
			}
		case "esc":
			m.SubscriptionSearch = ""
			m.Status = "" // Help text is in the view
		default:
			// Handle type-ahead search for subscriptions
			key := msg.String()
			if isSearchKey(key) {
				m.SubscriptionSearch += key
				// Find and select the subscription that matches the search
				searchLower := strings.ToLower(m.SubscriptionSearch)
				for i, sub := range m.Subscriptions {
					if strings.Contains(strings.ToLower(sub.Name), searchLower) {
						m.Cursor = i
						break
					}
				}
				m.Status = "" // Search query is shown in the view
			}
		}

	case StepSelectAssignment:
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Assignments)-1 {
				m.Cursor++
			}
		case "backspace":
			if m.AssignmentSearch != "" {
				// Delete the last character from the search buffer and re-jump
				m.AssignmentSearch = m.AssignmentSearch[:len(m.AssignmentSearch)-1]
				if m.AssignmentSearch != "" {
					if idx := m.firstAssignmentMatch(m.AssignmentSearch); idx >= 0 {
						m.Cursor = idx
					}
				}
				m.Status = "" // Search query is shown in the view
				return nil
			}
			// Empty search: go back to subscription selection
			m.Step = StepSelectSubscription
			m.Cursor = m.SelectedSubscription
			if m.Cursor < 0 {
				m.Cursor = 0
			}
			m.SelectedSubscription = -1
			m.Status = "" // Help text is in the view
			return nil
		case "esc":
			m.AssignmentSearch = ""
			m.Status = "" // Help text is in the view
		case "enter":
			if len(m.Assignments) == 0 {
				return nil
			}
			// Check if the assignment's policy definition is blocked
			assign := m.Assignments[m.Cursor]
			if m.IsDefinitionBlocked(assign.PolicyDefinitionID) {
				m.Status = "This policy assignment is blocked and cannot be exempted."
				return nil
			}
			m.SelectedAssignment = m.Cursor
			m.AssignmentSearch = ""
			m.Step = StepLoadingAssignmentDefinitions
			m.Status = "" // Loading state shown in view
			return fetchAssignmentDefinitionsCmd(m.ctx, m.azureClient, m.CurrentAssignment())
		default:
			// Handle type-ahead search for assignments
			key := msg.String()
			if isSearchKey(key) {
				m.AssignmentSearch += key
				if idx := m.firstAssignmentMatch(m.AssignmentSearch); idx >= 0 {
					m.Cursor = idx
				}
				m.Status = "" // Search query is shown in the view
			}
		}

	case StepAssignmentScope:
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < 1 {
				m.Cursor++
			}
		case "backspace":
			// Go back to assignment selection
			m.Step = StepSelectAssignment
			m.Cursor = m.SelectedAssignment
			if m.Cursor < 0 {
				m.Cursor = 0
			}
			m.SelectedAssignment = -1
			m.Status = "" // Help text is in the view
			return nil
		case "enter":
			if m.Cursor == 0 {
				m.PartialExemption = false
				m.Step = StepLoadingResourceGroups
				m.Status = "" // Loading state shown in view
				return fetchResourceGroupsCmd(m.ctx, m.azureClient, m.CurrentSubscription())
			}
			m.PartialExemption = true
			m.Step = StepSelectDefinitions
			m.DefinitionSearch = ""
			m.Cursor = 0
			m.Status = "" // Help text is in the view
			return nil
		}

	case StepSelectDefinitions:
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.AssignmentDefinitions)-1 {
				m.Cursor++
			}
		case "backspace":
			if m.DefinitionSearch != "" {
				// Delete the last character from the search buffer and re-jump
				m.DefinitionSearch = m.DefinitionSearch[:len(m.DefinitionSearch)-1]
				if m.DefinitionSearch != "" {
					if idx := m.firstDefinitionMatch(m.DefinitionSearch); idx >= 0 {
						m.Cursor = idx
					}
				}
				m.Status = "" // Search query is shown in the view
				return nil
			}
			// Empty search: go back to assignment scope selection
			m.Step = StepAssignmentScope
			m.Cursor = 1 // "Exempt specific definitions" was selected
			m.SelectedDefinitionIDs = make(map[string]bool)
			m.Status = "" // Help text is in the view
			return nil
		case "esc":
			m.DefinitionSearch = ""
			m.Status = "" // Help text is in the view
		case " ":
			if len(m.AssignmentDefinitions) == 0 {
				return nil
			}
			ref := m.AssignmentDefinitions[m.Cursor]
			// Check if the definition is blocked
			if m.IsDefinitionBlocked(ref.PolicyDefinitionID) {
				m.Status = "This policy definition is blocked and cannot be exempted."
				return nil
			}
			if m.SelectedDefinitionIDs[ref.ReferenceID] {
				delete(m.SelectedDefinitionIDs, ref.ReferenceID)
			} else {
				m.SelectedDefinitionIDs[ref.ReferenceID] = true
			}
			m.Status = "" // Clear any previous status
		case "enter":
			if len(m.AssignmentDefinitions) == 0 {
				return nil
			}
			if len(m.SelectedDefinitionIDs) == 0 {
				m.Status = "Select at least one definition or go back."
				return nil
			}
			m.DefinitionSearch = ""
			m.Step = StepLoadingResourceGroups
			m.Status = "" // Loading state shown in view
			return fetchResourceGroupsCmd(m.ctx, m.azureClient, m.CurrentSubscription())
		default:
			// Handle type-ahead search for definitions.
			// Space is reserved for toggling, so it is excluded from the search whitelist.
			key := msg.String()
			if isSearchKey(key) && key != " " {
				m.DefinitionSearch += key
				if idx := m.firstDefinitionMatch(m.DefinitionSearch); idx >= 0 {
					m.Cursor = idx
				}
				m.Status = "" // Search query is shown in the view
			}
		}

	case StepSelectResourceGroup:
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.ResourceGroups)-1 {
				m.Cursor++
			}
		case "backspace":
			// Go back to the appropriate step
			if m.PartialExemption {
				m.Step = StepSelectDefinitions
				m.DefinitionSearch = ""
				m.Cursor = 0
				m.Status = "" // Help text is in the view
			} else if len(m.AssignmentDefinitions) > 1 {
				m.Step = StepAssignmentScope
				m.Cursor = 0
				m.Status = "" // Help text is in the view
			} else {
				m.Step = StepSelectAssignment
				m.Cursor = m.SelectedAssignment
				if m.Cursor < 0 {
					m.Cursor = 0
				}
				m.SelectedAssignment = -1
				m.Status = "" // Help text is in the view
			}
			return nil
		case "enter":
			if len(m.ResourceGroups) == 0 {
				return nil
			}
			m.SelectedResourceGroup = m.Cursor
			m.Step = StepTicket
			m.TicketInput.SetValue("")
			m.TicketInput.Focus()
			m.Status = "" // Help text is in the view
			return nil
		}

	case StepTicket:
		// Check for backspace when input is empty to go back
		if msg.Type == tea.KeyBackspace && m.TicketInput.Value() == "" {
			m.Step = StepSelectResourceGroup
			m.Cursor = m.SelectedResourceGroup
			if m.Cursor < 0 {
				m.Cursor = 0
			}
			m.SelectedResourceGroup = -1
			m.TicketInput.Blur()
			m.Status = "" // Help text is in the view
			return nil
		}
		var textCmd tea.Cmd
		m.TicketInput, textCmd = m.TicketInput.Update(msg)
		if msg.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.TicketInput.Value())
			if value == "" {
				m.Status = "A ticket number is required."
				return textCmd
			}
			m.Ticket = value
			m.Step = StepUsers
			m.TicketInput.Blur()
			m.UserInput.SetValue("")
			m.UserInput.Focus()
			m.Status = "" // Help text is in the view
			return textCmd
		}
		return textCmd

	case StepUsers:
		// Check for backspace when input is empty to go back
		if msg.Type == tea.KeyBackspace && m.UserInput.Value() == "" {
			m.Step = StepTicket
			m.UserInput.Blur()
			m.TicketInput.SetValue(m.Ticket)
			m.TicketInput.Focus()
			m.Status = "" // Help text is in the view
			return nil
		}
		var textCmd tea.Cmd
		m.UserInput, textCmd = m.UserInput.Update(msg)
		if msg.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.UserInput.Value())
			if value == "" {
				m.Status = "At least one requester name is required."
				return textCmd
			}
			m.RequestUser = value
			m.Step = StepExpirationChoice
			m.UserInput.Blur()
			m.Cursor = 0
			m.Status = "" // Help text is in the view
			return textCmd
		}
		return textCmd

	case StepExpirationChoice:
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < 1 {
				m.Cursor++
			}
		case "backspace":
			// Go back to users step
			m.Step = StepUsers
			m.UserInput.SetValue(m.RequestUser)
			m.UserInput.Focus()
			m.Status = "" // Help text is in the view
			return nil
		case "enter":
			if m.Cursor == 0 {
				// Unlimited
				m.ExpirationDate = ""
				m.Step = StepConfirm
				m.Status = "" // Help text is in the view
			} else {
				// Set Date
				m.Step = StepExpirationDate
				m.ExpirationInput.SetValue(time.Now().AddDate(0, 0, 30).Format("2006-01-02"))
				m.ExpirationInput.Focus()
				m.Status = "" // Help text is in the view
			}
			return nil
		}

	case StepExpirationDate:
		// Check for backspace when input is empty to go back
		if msg.Type == tea.KeyBackspace && m.ExpirationInput.Value() == "" {
			m.Step = StepExpirationChoice
			m.Cursor = 1 // "Set expiration date" was selected
			m.ExpirationInput.Blur()
			m.Status = "" // Help text is in the view
			return nil
		}
		var textCmd tea.Cmd
		m.ExpirationInput, textCmd = m.ExpirationInput.Update(msg)
		if msg.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.ExpirationInput.Value())
			if value == "" {
				m.Status = "Expiration date is required."
				return textCmd
			}
			// Simple validation for YYYY-MM-DD
			_, err := time.Parse("2006-01-02", value)
			if err != nil {
				m.Status = "Invalid date format. Use YYYY-MM-DD."
				return textCmd
			}
			m.ExpirationDate = value
			m.Step = StepConfirm
			m.ExpirationInput.Blur()
			m.Status = "" // Help text is in the view
			return textCmd
		}
		return textCmd

	case StepConfirm:
		switch msg.String() {
		case "backspace":
			// Go back to the expiration choice step
			m.Step = StepExpirationChoice
			if m.ExpirationDate == "" {
				m.Cursor = 0 // Unlimited was selected
			} else {
				m.Cursor = 1 // Set expiration date was selected
			}
			m.Status = "" // Help text is in the view
			return nil
		case "enter":
			if m.SelectedAssignment < 0 || m.Ticket == "" || m.RequestUser == "" || m.SelectedSubscription < 0 || m.SelectedResourceGroup < 0 {
				m.Status = "Missing information. Use q to abort."
				return nil
			}
			m.Step = StepCreating
			assign := m.CurrentAssignment()
			rg := m.ResourceGroups[m.SelectedResourceGroup]
			sub := m.CurrentSubscription()
			m.Status = "" // Loading state shown in view
			return createExemptionCmd(m.ctx, m.azureClient, rg.ID, rg.Name, sub.Name, assign, m.SelectedDefinitionIDs, m.Ticket, m.RequestUser, m.ExpirationDate)
		}

	case StepError, StepLoadingAssignmentDefinitions, StepLoadingAssignments, StepLoadingSubscriptions, StepLoadingResourceGroups, StepCreating:
		// No interactive keys beyond quit for these states.
	case StepDone:
		// Allow creating a new exemption by pressing Enter
		if msg.Type == tea.KeyEnter {
			return m.Reset()
		}
	}

	return nil
}

// isSearchKey reports whether a key press should be appended to a type-ahead
// search buffer. Only single printable characters in the whitelist
// [a-zA-Z0-9 _-] are accepted; this mirrors the characters allowed when
// searching subscriptions.
func isSearchKey(key string) bool {
	if len(key) != 1 {
		return false
	}
	c := key[0]
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == ' ' || c == '-' || c == '_'
}
