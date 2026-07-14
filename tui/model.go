package tui

import (
	"context"
	"strings"

	"github.com/Lukas-Klein/azexempt/azure"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Step int

const (
	StepLoadingSubscriptions Step = iota
	StepSelectSubscription
	StepLoadingAssignments
	StepSelectAssignment
	StepLoadingAssignmentDefinitions
	StepAssignmentScope
	StepSelectDefinitions
	StepLoadingResourceGroups
	StepSelectResourceGroup
	StepTicket
	StepUsers
	StepExpirationChoice
	StepExpirationDate
	StepConfirm
	StepCreating
	StepDone
	StepError
)

type Model struct {
	ctx         context.Context
	azureClient azureClient

	Step   Step
	Status string
	Err    error

	Subscriptions         []azure.Subscription
	Assignments           []azure.PolicyAssignment
	AssignmentDefinitions []azure.PolicyDefinitionRef
	ResourceGroups        []azure.ResourceGroup
	SelectedDefinitionIDs map[string]bool
	Cursor                int
	SelectedSubscription  int
	SelectedAssignment    int
	SelectedResourceGroup int
	PartialExemption      bool

	TicketInput     textinput.Model
	UserInput       textinput.Model
	ExpirationInput textinput.Model

	Ticket         string
	RequestUser    string
	ExpirationDate string

	CreateOutput string

	// SubscriptionSearch is the type-ahead search buffer for subscription selection
	SubscriptionSearch string

	// AssignmentSearch is the type-ahead search buffer for policy assignment selection
	AssignmentSearch string

	// DefinitionSearch is the type-ahead search buffer for policy definition selection
	DefinitionSearch string

	// BlockedDefinitionIDs contains policy definition IDs that cannot be exempted.
	// These definitions appear greyed out and are non-selectable in the UI.
	BlockedDefinitionIDs map[string]bool
}

func NewModel(ctx context.Context, client azureClient, blockedDefinitionIDs map[string]bool) *Model {
	ticketInput := textinput.New()
	ticketInput.Placeholder = "e.g. INC123456"
	ticketInput.Prompt = "Ticket> "
	ticketInput.CharLimit = 128
	ticketInput.Blur()

	userInput := textinput.New()
	userInput.Placeholder = "Comma-separated requester names"
	userInput.Prompt = "Users> "
	userInput.CharLimit = 256
	userInput.Blur()

	expirationInput := textinput.New()
	expirationInput.Placeholder = "YYYY-MM-DD"
	expirationInput.Prompt = "Expires on> "
	expirationInput.CharLimit = 10
	expirationInput.Blur()

	if blockedDefinitionIDs == nil {
		blockedDefinitionIDs = make(map[string]bool)
	}

	return &Model{
		ctx:                   ctx,
		azureClient:           client,
		Step:                  StepLoadingSubscriptions,
		SelectedSubscription:  -1,
		SelectedAssignment:    -1,
		SelectedResourceGroup: -1,
		SelectedDefinitionIDs: make(map[string]bool),
		BlockedDefinitionIDs:  blockedDefinitionIDs,
		TicketInput:           ticketInput,
		UserInput:             userInput,
		ExpirationInput:       expirationInput,
	}
}

func (m *Model) Init() tea.Cmd {
	return fetchSubscriptionsCmd(m.ctx, m.azureClient)
}

func (m *Model) CurrentSubscription() azure.Subscription {
	if m.SelectedSubscription >= 0 && m.SelectedSubscription < len(m.Subscriptions) {
		return m.Subscriptions[m.SelectedSubscription]
	}
	if len(m.Subscriptions) == 0 {
		return azure.Subscription{}
	}
	return m.Subscriptions[0]
}

func (m *Model) CurrentAssignment() azure.PolicyAssignment {
	if m.SelectedAssignment >= 0 && m.SelectedAssignment < len(m.Assignments) {
		return m.Assignments[m.SelectedAssignment]
	}
	if len(m.Assignments) == 0 {
		return azure.PolicyAssignment{}
	}
	return m.Assignments[0]
}

func (m *Model) Fail(err error) (tea.Model, tea.Cmd) {
	m.Err = err
	m.Step = StepError
	m.Status = ""
	return m, nil
}

// Reset resets the model to start a new exemption creation flow
func (m *Model) Reset() tea.Cmd {
	m.Step = StepLoadingSubscriptions
	m.Status = ""
	m.Err = nil
	m.Cursor = 0
	m.SelectedSubscription = -1
	m.SelectedAssignment = -1
	m.SelectedResourceGroup = -1
	m.Assignments = nil
	m.AssignmentDefinitions = nil
	m.ResourceGroups = nil
	m.SelectedDefinitionIDs = make(map[string]bool)
	m.PartialExemption = false
	m.Ticket = ""
	m.RequestUser = ""
	m.ExpirationDate = ""
	m.CreateOutput = ""
	m.SubscriptionSearch = ""
	m.AssignmentSearch = ""
	m.DefinitionSearch = ""

	m.TicketInput.SetValue("")
	m.TicketInput.Blur()
	m.UserInput.SetValue("")
	m.UserInput.Blur()
	m.ExpirationInput.SetValue("")
	m.ExpirationInput.Blur()

	return fetchSubscriptionsCmd(m.ctx, m.azureClient)
}

// IsDefinitionBlocked returns true if the given policy definition ID is blocked from exemption.
// The comparison is case-insensitive.
func (m *Model) IsDefinitionBlocked(policyDefinitionID string) bool {
	return m.BlockedDefinitionIDs[strings.ToLower(policyDefinitionID)]
}

// firstAssignmentMatch returns the index of the first non-blocked assignment whose
// display label contains the (case-insensitive) query, or -1 if none matches.
func (m *Model) firstAssignmentMatch(query string) int {
	q := strings.ToLower(query)
	for i, assign := range m.Assignments {
		if m.IsDefinitionBlocked(assign.PolicyDefinitionID) {
			continue
		}
		if strings.Contains(strings.ToLower(assign.DisplayLabel()), q) {
			return i
		}
	}
	return -1
}

// firstDefinitionMatch returns the index of the first non-blocked assignment definition
// whose display name contains the (case-insensitive) query, or -1 if none matches.
func (m *Model) firstDefinitionMatch(query string) int {
	q := strings.ToLower(query)
	for i, ref := range m.AssignmentDefinitions {
		if m.IsDefinitionBlocked(ref.PolicyDefinitionID) {
			continue
		}
		if strings.Contains(strings.ToLower(ref.DisplayName), q) {
			return i
		}
	}
	return -1
}
