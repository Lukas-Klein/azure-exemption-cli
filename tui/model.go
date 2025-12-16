package tui

import (
	"context"

	"github.com/Lukas-Klein/azure-exemption-cli/azure"
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
	azureClient *azure.Client

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
}

func NewModel(ctx context.Context, client *azure.Client) *Model {
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

	return &Model{
		ctx:                   ctx,
		azureClient:           client,
		Step:                  StepLoadingSubscriptions,
		SelectedSubscription:  -1,
		SelectedAssignment:    -1,
		SelectedResourceGroup: -1,
		SelectedDefinitionIDs: make(map[string]bool),
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
