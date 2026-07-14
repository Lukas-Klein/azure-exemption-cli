package tui

import (
	"errors"
	"strings"
	"testing"
)

func TestViewEveryStep(t *testing.T) {
	m := populatedModel()
	m.PartialExemption = true
	m.SelectedDefinitionIDs["ref-a"] = true
	m.CreateOutput = "created output"
	m.Err = errors.New("failed")
	tests := []struct {
		step Step
		want string
	}{
		{StepLoadingSubscriptions, "Retrieving subscriptions"},
		{StepSelectSubscription, "Select the subscription"},
		{StepLoadingAssignments, "Loading policy assignments"},
		{StepSelectAssignment, "Policy assignments for subscription"},
		{StepLoadingAssignmentDefinitions, "Loading assignment details"},
		{StepAssignmentScope, "multiple policy definitions"},
		{StepSelectDefinitions, "Select the policy definitions"},
		{StepLoadingResourceGroups, "Loading resource groups"},
		{StepSelectResourceGroup, "Select the scope"},
		{StepTicket, "tracking ticket"},
		{StepUsers, "Who is requesting"},
		{StepExpirationChoice, "set an expiration date"},
		{StepExpirationDate, "Enter the expiration date"},
		{StepConfirm, "Review Exemption Details"},
		{StepCreating, "Creating policy exemption"},
		{StepDone, "created output"},
		{StepError, "failed"},
	}
	for _, tt := range tests {
		m.Step = tt.step
		if got := m.View(); !strings.Contains(got, tt.want) {
			t.Errorf("View(step %d) does not contain %q:\n%s", tt.step, tt.want, got)
		}
	}

	m.Step = StepDone
	m.CreateOutput = ""
	if got := m.View(); !strings.Contains(got, "No output returned") {
		t.Fatalf("empty done view = %q", got)
	}
	m.Step = StepSelectAssignment
	m.BlockedDefinitionIDs[strings.ToLower(m.Assignments[0].PolicyDefinitionID)] = true
	if got := m.View(); !strings.Contains(got, "[blocked]") {
		t.Fatalf("blocked assignment view = %q", got)
	}
	m.Status = "validation failed"
	if got := m.View(); !strings.Contains(got, "validation failed") {
		t.Fatalf("status view = %q", got)
	}
}

func TestVisibleRange(t *testing.T) {
	tests := []struct {
		cursor, total, limit int
		start, end           int
	}{
		{0, 0, 15, 0, 0},
		{0, 5, 15, 0, 5},
		{0, 20, 0, 0, 20},
		{0, 20, 5, 0, 5},
		{10, 20, 5, 8, 13},
		{19, 20, 5, 15, 20},
	}
	for _, tt := range tests {
		start, end := visibleRange(tt.cursor, tt.total, tt.limit)
		if start != tt.start || end != tt.end {
			t.Errorf("visibleRange(%d, %d, %d) = %d, %d; want %d, %d", tt.cursor, tt.total, tt.limit, start, end, tt.start, tt.end)
		}
	}
}

func TestFormatHint(t *testing.T) {
	got := formatHint("Enter", "select")
	if !strings.Contains(got, "Enter") || !strings.Contains(got, "select") {
		t.Fatalf("formatHint() = %q", got)
	}
}
