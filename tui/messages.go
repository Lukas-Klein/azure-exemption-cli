package tui

import (
	"context"

	"github.com/Lukas-Klein/azure-exemption-cli/azure"
	tea "github.com/charmbracelet/bubbletea"
)

type subscriptionsLoadedMsg struct {
	subscriptions []azure.Subscription
	err           error
}

type assignmentsLoadedMsg struct {
	assignments []azure.PolicyAssignment
	err         error
}

type assignmentDefinitionsLoadedMsg struct {
	definitions []azure.PolicyDefinitionRef
	err         error
}

type resourceGroupsLoadedMsg struct {
	resourceGroups []azure.ResourceGroup
	err            error
}

type exemptionCreatedMsg struct {
	output string
	err    error
}

func fetchSubscriptionsCmd(ctx context.Context, client *azure.Client) tea.Cmd {
	return func() tea.Msg {
		subs, err := client.ListSubscriptions(ctx)
		return subscriptionsLoadedMsg{subscriptions: subs, err: err}
	}
}

func fetchAssignmentsCmd(ctx context.Context, client *azure.Client, sub azure.Subscription) tea.Cmd {
	return func() tea.Msg {
		assignments, err := client.ListAssignments(ctx, sub.ShortID())
		return assignmentsLoadedMsg{assignments: assignments, err: err}
	}
}

func fetchAssignmentDefinitionsCmd(ctx context.Context, client *azure.Client, assignment azure.PolicyAssignment) tea.Cmd {
	return func() tea.Msg {
		definitions, err := client.ListAssignmentDefinitions(ctx, assignment)
		return assignmentDefinitionsLoadedMsg{definitions: definitions, err: err}
	}
}

func fetchResourceGroupsCmd(ctx context.Context, client *azure.Client, sub azure.Subscription) tea.Cmd {
	return func() tea.Msg {
		rgs, err := client.ListResourceGroups(ctx, sub.ShortID())
		return resourceGroupsLoadedMsg{resourceGroups: rgs, err: err}
	}
}

func createExemptionCmd(ctx context.Context, client *azure.Client, scope string, assignment azure.PolicyAssignment, selectedDefinitionIDs map[string]bool, ticket, users, expirationDate string) tea.Cmd {
	return func() tea.Msg {
		var refs []string
		for ref := range selectedDefinitionIDs {
			refs = append(refs, ref)
		}
		output, err := client.CreateExemption(ctx, scope, assignment, refs, ticket, users, expirationDate)
		return exemptionCreatedMsg{output: output, err: err}
	}
}
