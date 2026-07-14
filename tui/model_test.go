package tui

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Lukas-Klein/azure-exemption-cli/azure"
)

func TestNewModelAndSelectionHelpers(t *testing.T) {
	m := NewModel(context.Background(), &fakeAzureClient{}, nil)
	if m.Step != StepLoadingSubscriptions || m.SelectedSubscription != -1 || m.SelectedAssignment != -1 || m.SelectedResourceGroup != -1 {
		t.Fatalf("unexpected initial selection state: %#v", m)
	}
	if m.BlockedDefinitionIDs == nil || m.SelectedDefinitionIDs == nil {
		t.Fatal("selection maps must be initialized")
	}
	if m.TicketInput.Prompt != "Ticket> " || m.TicketInput.CharLimit != 128 || m.UserInput.CharLimit != 256 || m.ExpirationInput.CharLimit != 10 {
		t.Fatal("text inputs were not configured")
	}

	m.Subscriptions = []azure.Subscription{{ID: "first"}, {ID: "second"}}
	m.Assignments = []azure.PolicyAssignment{{Name: "first"}, {Name: "second"}}
	if got := m.CurrentSubscription(); got.ID != "first" {
		t.Fatalf("default subscription = %#v", got)
	}
	if got := m.CurrentAssignment(); got.Name != "first" {
		t.Fatalf("default assignment = %#v", got)
	}
	m.SelectedSubscription, m.SelectedAssignment = 1, 1
	if m.CurrentSubscription().ID != "second" || m.CurrentAssignment().Name != "second" {
		t.Fatal("valid selected items were not returned")
	}
	m.SelectedSubscription, m.SelectedAssignment = 99, 99
	if m.CurrentSubscription().ID != "first" || m.CurrentAssignment().Name != "first" {
		t.Fatal("invalid selection should fall back to first item")
	}
}

func TestFailAndReset(t *testing.T) {
	blocked := map[string]bool{"blocked": true}
	m := NewModel(context.Background(), &fakeAzureClient{}, blocked)
	m.Step = StepDone
	m.Status = "status"
	m.Err = errors.New("old")
	m.Subscriptions = []azure.Subscription{{ID: "preserved"}}
	m.Assignments = []azure.PolicyAssignment{{Name: "a"}}
	m.AssignmentDefinitions = []azure.PolicyDefinitionRef{{ReferenceID: "r"}}
	m.ResourceGroups = []azure.ResourceGroup{{Name: "rg"}}
	m.SelectedDefinitionIDs["r"] = true
	m.PartialExemption = true
	m.Ticket, m.RequestUser, m.ExpirationDate, m.CreateOutput = "T", "U", "D", "O"
	m.SubscriptionSearch, m.AssignmentSearch, m.DefinitionSearch = "s", "a", "d"
	m.TicketInput.SetValue("T")
	m.UserInput.SetValue("U")
	m.ExpirationInput.SetValue("D")

	errBoom := errors.New("boom")
	model, cmd := m.Fail(errBoom)
	if model != m || cmd != nil || m.Step != StepError || !errors.Is(m.Err, errBoom) || m.Status != "" {
		t.Fatalf("Fail() left unexpected state: %#v", m)
	}

	cmd = m.Reset()
	if cmd == nil || m.Step != StepLoadingSubscriptions || m.Err != nil || m.SelectedSubscription != -1 || m.SelectedAssignment != -1 || m.SelectedResourceGroup != -1 {
		t.Fatalf("Reset() basic state = %#v", m)
	}
	if m.Assignments != nil || m.AssignmentDefinitions != nil || m.ResourceGroups != nil || m.PartialExemption || len(m.SelectedDefinitionIDs) != 0 {
		t.Fatal("Reset() did not clear Azure selections")
	}
	if m.Ticket != "" || m.RequestUser != "" || m.ExpirationDate != "" || m.CreateOutput != "" || m.TicketInput.Value() != "" || m.UserInput.Value() != "" || m.ExpirationInput.Value() != "" {
		t.Fatal("Reset() did not clear form values")
	}
	if !reflect.DeepEqual(m.BlockedDefinitionIDs, blocked) || len(m.Subscriptions) != 1 {
		t.Fatal("Reset() should retain configuration and cached subscriptions")
	}
}

func TestBlockedAndSearchHelpers(t *testing.T) {
	m := NewModel(context.Background(), &fakeAzureClient{}, map[string]bool{"/definitions/blocked": true})
	if !m.IsDefinitionBlocked("/DEFINITIONS/BLOCKED") || m.IsDefinitionBlocked("allowed") {
		t.Fatal("blocked lookup is not case-insensitive")
	}
	m.Assignments = []azure.PolicyAssignment{
		{DisplayName: "Blocked match", PolicyDefinitionID: "/definitions/blocked"},
		{DisplayName: "Allowed Match", PolicyDefinitionID: "/definitions/allowed"},
	}
	if got := m.firstAssignmentMatch("MATCH"); got != 1 {
		t.Fatalf("firstAssignmentMatch() = %d", got)
	}
	if got := m.firstAssignmentMatch("missing"); got != -1 {
		t.Fatalf("missing assignment match = %d", got)
	}
	m.AssignmentDefinitions = []azure.PolicyDefinitionRef{
		{DisplayName: "Blocked", PolicyDefinitionID: "/definitions/blocked"},
		{DisplayName: "Allowed", PolicyDefinitionID: "/definitions/allowed"},
	}
	if got := m.firstDefinitionMatch("ed"); got != 1 {
		t.Fatalf("firstDefinitionMatch() = %d", got)
	}
}
