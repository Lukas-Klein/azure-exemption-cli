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
		m.Status = "Use ↑/↓ to highlight a subscription and press Enter to continue."
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
		m.Cursor = 0
		m.Step = StepSelectAssignment
		m.Status = "Use ↑/↓ to highlight an assignment and press Enter to continue."
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
			m.Status = "Exempt entire assignment or select specific definitions?"
		} else {
			m.PartialExemption = false
			m.Step = StepLoadingResourceGroups
			m.Status = "Loading resource groups..."
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
		m.Status = "Select the scope for the exemption (Subscription or Resource Group)."
		return m, nil

	case exemptionCreatedMsg:
		if msg.err != nil {
			return m.Fail(msg.err)
		}
		m.CreateOutput = msg.output
		m.Step = StepDone
		m.Status = "Exemption created successfully. Press q to exit."
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
			m.Step = StepLoadingAssignments
			sub := m.CurrentSubscription()
			m.Status = fmt.Sprintf("Fetching policy assignments for %s...", sub.Name)
			return fetchAssignmentsCmd(m.ctx, m.azureClient, sub)
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
		case "enter":
			if len(m.Assignments) == 0 {
				return nil
			}
			m.SelectedAssignment = m.Cursor
			m.Step = StepLoadingAssignmentDefinitions
			assign := m.CurrentAssignment()
			m.Status = fmt.Sprintf("Fetching assignment details for %s...", assign.DisplayLabel())
			return fetchAssignmentDefinitionsCmd(m.ctx, m.azureClient, assign)
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
		case "enter":
			if m.Cursor == 0 {
				m.PartialExemption = false
				m.Step = StepLoadingResourceGroups
				m.Status = "Loading resource groups..."
				return fetchResourceGroupsCmd(m.ctx, m.azureClient, m.CurrentSubscription())
			}
			m.PartialExemption = true
			m.Step = StepSelectDefinitions
			m.Cursor = 0
			m.Status = "Select definitions to exempt (space to toggle, Enter to continue)."
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
		case " ":
			if len(m.AssignmentDefinitions) == 0 {
				return nil
			}
			ref := m.AssignmentDefinitions[m.Cursor]
			if m.SelectedDefinitionIDs[ref.ReferenceID] {
				delete(m.SelectedDefinitionIDs, ref.ReferenceID)
			} else {
				m.SelectedDefinitionIDs[ref.ReferenceID] = true
			}
		case "enter":
			if len(m.AssignmentDefinitions) == 0 {
				return nil
			}
			if len(m.SelectedDefinitionIDs) == 0 {
				m.Status = "Select at least one definition or choose full assignment."
				return nil
			}
			m.Step = StepLoadingResourceGroups
			m.Status = "Loading resource groups..."
			return fetchResourceGroupsCmd(m.ctx, m.azureClient, m.CurrentSubscription())
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
		case "enter":
			if len(m.ResourceGroups) == 0 {
				return nil
			}
			m.SelectedResourceGroup = m.Cursor
			m.Step = StepTicket
			m.TicketInput.SetValue("")
			m.TicketInput.Focus()
			m.Status = "Provide the tracking ticket number linked to this exemption:"
			return nil
		}

	case StepTicket:
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
			m.Status = "Who is requesting this exemption? Provide one or more names."
			return textCmd
		}
		return textCmd

	case StepUsers:
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
			m.Status = "Set an expiration date for this exemption?"
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
		case "enter":
			if m.Cursor == 0 {
				// Unlimited
				m.ExpirationDate = ""
				m.Step = StepConfirm
				m.Status = "Review the summary and press Enter to create the exemption."
			} else {
				// Set Date
				m.Step = StepExpirationDate
				m.ExpirationInput.SetValue(time.Now().AddDate(0, 0, 30).Format("2006-01-02"))
				m.ExpirationInput.Focus()
				m.Status = "Enter expiration date (YYYY-MM-DD):"
			}
			return nil
		}

	case StepExpirationDate:
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
				m.Status = "Invalid date format. Please use YYYY-MM-DD."
				return textCmd
			}
			m.ExpirationDate = value
			m.Step = StepConfirm
			m.ExpirationInput.Blur()
			m.Status = "Review the summary and press Enter to create the exemption."
			return textCmd
		}
		return textCmd

	case StepConfirm:
		if msg.Type == tea.KeyEnter {
			if m.SelectedAssignment < 0 || m.Ticket == "" || m.RequestUser == "" || m.SelectedSubscription < 0 || m.SelectedResourceGroup < 0 {
				m.Status = "Missing information. Use q to abort."
				return nil
			}
			m.Step = StepCreating
			assign := m.CurrentAssignment()
			rg := m.ResourceGroups[m.SelectedResourceGroup]
			m.Status = "Creating Azure Policy exemption..."
			return createExemptionCmd(m.ctx, m.azureClient, rg.ID, assign, m.SelectedDefinitionIDs, m.Ticket, m.RequestUser, m.ExpirationDate)
		}

	case StepError, StepDone, StepLoadingAssignmentDefinitions, StepLoadingAssignments, StepLoadingSubscriptions, StepLoadingResourceGroups, StepCreating:
		// No interactive keys beyond quit for these states.
	}

	return nil
}
