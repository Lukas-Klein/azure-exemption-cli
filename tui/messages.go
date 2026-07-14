package tui

import (
	"context"
	"sort"

	"github.com/Lukas-Klein/azure-exemption-cli/azure"
	tea "github.com/charmbracelet/bubbletea"
)

type azureClient interface {
	ListSubscriptions(context.Context) ([]azure.Subscription, error)
	ListAssignments(context.Context, string) ([]azure.PolicyAssignment, error)
	ListAssignmentDefinitions(context.Context, azure.PolicyAssignment) ([]azure.PolicyDefinitionRef, error)
	ListResourceGroups(context.Context, string) ([]azure.ResourceGroup, error)
	CreateExemption(context.Context, string, string, string, azure.PolicyAssignment, []string, string, string, string) (string, error)
}

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

func fetchSubscriptionsCmd(ctx context.Context, client azureClient) tea.Cmd {
	return func() tea.Msg {
		subs, err := client.ListSubscriptions(ctx)
		return subscriptionsLoadedMsg{subscriptions: subs, err: err}
	}
}

func fetchAssignmentsCmd(ctx context.Context, client azureClient, sub azure.Subscription) tea.Cmd {
	return func() tea.Msg {
		assignments, err := client.ListAssignments(ctx, sub.ShortID())
		return assignmentsLoadedMsg{assignments: assignments, err: err}
	}
}

func fetchAssignmentDefinitionsCmd(ctx context.Context, client azureClient, assignment azure.PolicyAssignment) tea.Cmd {
	return func() tea.Msg {
		definitions, err := client.ListAssignmentDefinitions(ctx, assignment)
		return assignmentDefinitionsLoadedMsg{definitions: definitions, err: err}
	}
}

func fetchResourceGroupsCmd(ctx context.Context, client azureClient, sub azure.Subscription) tea.Cmd {
	return func() tea.Msg {
		rgs, err := client.ListResourceGroups(ctx, sub.ShortID())
		return resourceGroupsLoadedMsg{resourceGroups: rgs, err: err}
	}
}

func createExemptionCmd(ctx context.Context, client azureClient, scope string, scopeName string, subscriptionName string, assignment azure.PolicyAssignment, selectedDefinitionIDs map[string]bool, ticket, users, expirationDate string) tea.Cmd {
	return func() tea.Msg {
		var refs []string
		for ref := range selectedDefinitionIDs {
			refs = append(refs, ref)
		}
		sort.Strings(refs)
		output, err := client.CreateExemption(ctx, scope, scopeName, subscriptionName, assignment, refs, ticket, users, expirationDate)
		return exemptionCreatedMsg{output: output, err: err}
	}
}
