package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Lukas-Klein/azure-exemption-cli/azure"
	tea "github.com/charmbracelet/bubbletea"
)

func TestCompletePartialExemptionFlow(t *testing.T) {
	client := &fakeAzureClient{
		subscriptions: []azure.Subscription{{ID: "/subscriptions/sub-1", Name: "Production"}},
		assignments: []azure.PolicyAssignment{{
			ID: "/assignments/a", DisplayName: "Security baseline", PolicyDefinitionID: "/policySetDefinitions/set",
		}},
		definitions: []azure.PolicyDefinitionRef{
			{PolicyDefinitionID: "/definitions/one", ReferenceID: "ref-one", DisplayName: "First"},
			{PolicyDefinitionID: "/definitions/two", ReferenceID: "ref-two", DisplayName: "Second"},
		},
		resourceGroups: []azure.ResourceGroup{{ID: "/subscriptions/sub-1/resourceGroups/app", Name: "app"}},
		createOutput:   `{"created":true}`,
	}
	m := NewModel(context.Background(), client, nil)

	updateWith(t, m, m.Init()())
	assertStep(t, m, StepSelectSubscription)
	cmd := key(t, m, tea.KeyEnter)
	assertStep(t, m, StepLoadingAssignments)
	updateWith(t, m, cmd())
	assertStep(t, m, StepSelectAssignment)
	cmd = key(t, m, tea.KeyEnter)
	updateWith(t, m, cmd())
	assertStep(t, m, StepAssignmentScope)

	key(t, m, tea.KeyDown)
	key(t, m, tea.KeyEnter)
	assertStep(t, m, StepSelectDefinitions)
	key(t, m, tea.KeySpace)
	if !m.SelectedDefinitionIDs["ref-one"] {
		t.Fatal("definition was not selected")
	}
	cmd = key(t, m, tea.KeyEnter)
	updateWith(t, m, cmd())
	assertStep(t, m, StepSelectResourceGroup)
	if len(m.ResourceGroups) != 2 || m.ResourceGroups[0].Name != "Entire Subscription" {
		t.Fatalf("resource groups = %#v", m.ResourceGroups)
	}

	key(t, m, tea.KeyDown)
	key(t, m, tea.KeyEnter)
	m.TicketInput.SetValue(" INC123 ")
	key(t, m, tea.KeyEnter)
	m.UserInput.SetValue(" Ada, Linus ")
	key(t, m, tea.KeyEnter)
	key(t, m, tea.KeyEnter)
	assertStep(t, m, StepConfirm)
	cmd = key(t, m, tea.KeyEnter)
	assertStep(t, m, StepCreating)
	updateWith(t, m, cmd())
	assertStep(t, m, StepDone)
	if m.CreateOutput != client.createOutput || client.created.scopeName != "app" || client.created.ticket != "INC123" || client.created.users != "Ada, Linus" || len(client.created.refs) != 1 || client.created.refs[0] != "ref-one" {
		t.Fatalf("create result/call = %q, %#v", m.CreateOutput, client.created)
	}
}

func TestLoadedMessageBranches(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.Msg
		step Step
		err  string
	}{
		{"subscription error", subscriptionsLoadedMsg{err: errors.New("subs")}, StepError, "subs"},
		{"empty subscriptions", subscriptionsLoadedMsg{}, StepError, "no subscriptions"},
		{"assignment error", assignmentsLoadedMsg{err: errors.New("assign")}, StepError, "assign"},
		{"empty assignments", assignmentsLoadedMsg{}, StepError, "no policy assignments"},
		{"definition error", assignmentDefinitionsLoadedMsg{err: errors.New("defs")}, StepError, "defs"},
		{"resource group error", resourceGroupsLoadedMsg{err: errors.New("groups")}, StepError, "groups"},
		{"create error", exemptionCreatedMsg{err: errors.New("create")}, StepError, "create"},
		{"create success", exemptionCreatedMsg{output: "ok"}, StepDone, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := populatedModel()
			updateWith(t, m, tt.msg)
			assertStep(t, m, tt.step)
			if tt.err != "" && (m.Err == nil || !strings.Contains(m.Err.Error(), tt.err)) {
				t.Fatalf("error = %v, want containing %q", m.Err, tt.err)
			}
		})
	}

	m := populatedModel()
	cmd := updateWith(t, m, assignmentDefinitionsLoadedMsg{definitions: []azure.PolicyDefinitionRef{{ReferenceID: "one"}}})
	assertStep(t, m, StepLoadingResourceGroups)
	if cmd == nil {
		t.Fatal("single definition should load resource groups")
	}
}

func TestBlockedSelectionAndValidation(t *testing.T) {
	m := populatedModel()
	m.BlockedDefinitionIDs[strings.ToLower(m.Assignments[0].PolicyDefinitionID)] = true
	m.Step = StepSelectAssignment
	key(t, m, tea.KeyEnter)
	if m.Step != StepSelectAssignment || !strings.Contains(m.Status, "blocked") {
		t.Fatal("blocked assignment was selectable")
	}

	m.Step = StepSelectDefinitions
	m.Cursor = 0
	m.Status = ""
	m.BlockedDefinitionIDs[strings.ToLower(m.AssignmentDefinitions[0].PolicyDefinitionID)] = true
	key(t, m, tea.KeySpace)
	if len(m.SelectedDefinitionIDs) != 0 || !strings.Contains(m.Status, "blocked") {
		t.Fatal("blocked definition was selectable")
	}
	m.BlockedDefinitionIDs = map[string]bool{}
	key(t, m, tea.KeyEnter)
	if !strings.Contains(m.Status, "at least one") {
		t.Fatalf("empty definition validation = %q", m.Status)
	}

	m.Step = StepTicket
	m.TicketInput.Focus()
	m.TicketInput.SetValue("   ")
	key(t, m, tea.KeyEnter)
	if !strings.Contains(m.Status, "ticket") {
		t.Fatalf("ticket validation = %q", m.Status)
	}
	m.Step = StepUsers
	m.UserInput.Focus()
	m.UserInput.SetValue(" ")
	key(t, m, tea.KeyEnter)
	if !strings.Contains(m.Status, "requester") {
		t.Fatalf("user validation = %q", m.Status)
	}
	m.Step = StepExpirationDate
	m.ExpirationInput.Focus()
	m.ExpirationInput.SetValue("invalid")
	key(t, m, tea.KeyEnter)
	if !strings.Contains(m.Status, "Invalid date") {
		t.Fatalf("date validation = %q", m.Status)
	}
	m.Step = StepConfirm
	m.SelectedAssignment = -1
	key(t, m, tea.KeyEnter)
	if !strings.Contains(m.Status, "Missing information") {
		t.Fatalf("confirmation validation = %q", m.Status)
	}
}

func TestNavigationAndSearch(t *testing.T) {
	m := populatedModel()
	m.Step = StepSelectSubscription
	m.Subscriptions = append(m.Subscriptions, azure.Subscription{Name: "Zeta"})
	keyRune(t, m, 'z')
	if m.Cursor != 1 || m.SubscriptionSearch != "z" {
		t.Fatalf("subscription search = %d, %q", m.Cursor, m.SubscriptionSearch)
	}
	key(t, m, tea.KeyBackspace)
	if m.SubscriptionSearch != "" {
		t.Fatal("subscription search did not delete")
	}

	m.Step = StepSelectAssignment
	m.SelectedSubscription = 0
	m.AssignmentSearch = "sec"
	key(t, m, tea.KeyBackspace)
	if m.AssignmentSearch != "se" {
		t.Fatal("assignment search did not delete")
	}
	m.AssignmentSearch = ""
	key(t, m, tea.KeyBackspace)
	assertStep(t, m, StepSelectSubscription)

	m.Step = StepSelectDefinitions
	m.DefinitionSearch = "fir"
	key(t, m, tea.KeyBackspace)
	if m.DefinitionSearch != "fi" {
		t.Fatal("definition search did not delete")
	}
	m.DefinitionSearch = ""
	m.SelectedDefinitionIDs["ref"] = true
	key(t, m, tea.KeyBackspace)
	assertStep(t, m, StepAssignmentScope)
	if len(m.SelectedDefinitionIDs) != 0 || m.Cursor != 1 {
		t.Fatal("definition back navigation did not reset selection")
	}

	m.Step = StepSelectResourceGroup
	m.PartialExemption = true
	key(t, m, tea.KeyBackspace)
	assertStep(t, m, StepSelectDefinitions)
	m.Step = StepSelectResourceGroup
	m.PartialExemption = false
	key(t, m, tea.KeyBackspace)
	assertStep(t, m, StepAssignmentScope)

	m.Step = StepExpirationChoice
	m.RequestUser = "Ada"
	key(t, m, tea.KeyBackspace)
	assertStep(t, m, StepUsers)
	if m.UserInput.Value() != "Ada" {
		t.Fatal("user input was not restored")
	}
	m.Step = StepConfirm
	m.ExpirationDate = "2030-01-01"
	key(t, m, tea.KeyBackspace)
	if m.Step != StepExpirationChoice || m.Cursor != 1 {
		t.Fatal("dated confirmation back navigation is wrong")
	}
}

func TestQuitAndSearchKey(t *testing.T) {
	m := populatedModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return quit command")
	}
	for _, key := range []string{"a", "Z", "0", " ", "-", "_"} {
		if !isSearchKey(key) {
			t.Errorf("isSearchKey(%q) = false", key)
		}
	}
	for _, key := range []string{"", "ab", ".", "é", "enter"} {
		if isSearchKey(key) {
			t.Errorf("isSearchKey(%q) = true", key)
		}
	}
}

func populatedModel() *Model {
	m := NewModel(context.Background(), &fakeAzureClient{}, map[string]bool{})
	m.Subscriptions = []azure.Subscription{{ID: "sub", Name: "Sub"}}
	m.Assignments = []azure.PolicyAssignment{{ID: "/assignments/a", DisplayName: "Security", PolicyDefinitionID: "/definitions/a"}}
	m.AssignmentDefinitions = []azure.PolicyDefinitionRef{{PolicyDefinitionID: "/definitions/a", ReferenceID: "ref-a", DisplayName: "First"}, {PolicyDefinitionID: "/definitions/b", ReferenceID: "ref-b", DisplayName: "Second"}}
	m.ResourceGroups = []azure.ResourceGroup{{ID: "/subscriptions/sub", Name: "Entire Subscription"}}
	m.SelectedSubscription = 0
	m.SelectedAssignment = 0
	m.SelectedResourceGroup = 0
	m.Ticket = "INC1"
	m.RequestUser = "Ada"
	return m
}

func key(t *testing.T, m *Model, typ tea.KeyType) tea.Cmd {
	t.Helper()
	return updateWith(t, m, tea.KeyMsg{Type: typ})
}

func keyRune(t *testing.T, m *Model, r rune) tea.Cmd {
	t.Helper()
	return updateWith(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

func updateWith(t *testing.T, m *Model, msg tea.Msg) tea.Cmd {
	t.Helper()
	model, cmd := m.Update(msg)
	if model != m {
		t.Fatal("Update() returned a different model")
	}
	return cmd
}

func assertStep(t *testing.T, m *Model, want Step) {
	t.Helper()
	if m.Step != want {
		t.Fatalf("step = %v, want %v (status %q, error %v)", m.Step, want, m.Status, m.Err)
	}
}
