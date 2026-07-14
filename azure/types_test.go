package azure

import "testing"

func TestSubscriptionHelpers(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		scope string
		short string
	}{
		{name: "bare", id: "abc", scope: "/subscriptions/abc", short: "abc"},
		{name: "qualified", id: "/subscriptions/abc", scope: "/subscriptions/abc", short: "abc"},
		{name: "empty", scope: "/subscriptions/", short: ""},
		{name: "other resource ID", id: "/providers/example", scope: "/providers/example", short: "/providers/example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Subscription{ID: tt.id}
			if got := s.Scope(); got != tt.scope {
				t.Errorf("Scope() = %q, want %q", got, tt.scope)
			}
			if got := s.ShortID(); got != tt.short {
				t.Errorf("ShortID() = %q, want %q", got, tt.short)
			}
		})
	}
}

func TestPolicyAssignmentHelpers(t *testing.T) {
	p := PolicyAssignment{ID: "/subscriptions/s/policyAssignments/internal", Name: "internal", DisplayName: "Friendly"}
	if got := p.DisplayLabel(); got != "Friendly" {
		t.Fatalf("DisplayLabel() = %q", got)
	}
	if got := p.ShortID(); got != "internal" {
		t.Fatalf("ShortID() = %q", got)
	}
	p.DisplayName = ""
	if got := p.DisplayLabel(); got != "internal" {
		t.Fatalf("DisplayLabel() fallback = %q", got)
	}
	if got := (PolicyAssignment{}).ShortID(); got != "" {
		t.Fatalf("empty ShortID() = %q", got)
	}
}
