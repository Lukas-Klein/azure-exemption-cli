package tui

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Lukas-Klein/azure-exemption-cli/azure"
)

func TestFetchCommands(t *testing.T) {
	wantErr := errors.New("service failure")
	client := &fakeAzureClient{
		subscriptions:  []azure.Subscription{{ID: "sub"}},
		assignments:    []azure.PolicyAssignment{{ID: "assignment"}},
		definitions:    []azure.PolicyDefinitionRef{{ReferenceID: "ref"}},
		resourceGroups: []azure.ResourceGroup{{ID: "rg"}},
	}
	ctx := context.Background()

	subs := fetchSubscriptionsCmd(ctx, client)().(subscriptionsLoadedMsg)
	if !reflect.DeepEqual(subs.subscriptions, client.subscriptions) || subs.err != nil {
		t.Fatalf("subscriptions message = %#v", subs)
	}
	assignments := fetchAssignmentsCmd(ctx, client, azure.Subscription{ID: "/subscriptions/sub"})().(assignmentsLoadedMsg)
	if !reflect.DeepEqual(assignments.assignments, client.assignments) || client.assignmentSubscription != "sub" {
		t.Fatalf("assignments message = %#v, subscription = %q", assignments, client.assignmentSubscription)
	}
	assignment := azure.PolicyAssignment{ID: "assignment"}
	definitions := fetchAssignmentDefinitionsCmd(ctx, client, assignment)().(assignmentDefinitionsLoadedMsg)
	if !reflect.DeepEqual(definitions.definitions, client.definitions) || client.definitionAssignment.ID != assignment.ID {
		t.Fatalf("definitions message = %#v", definitions)
	}
	rgs := fetchResourceGroupsCmd(ctx, client, azure.Subscription{ID: "sub"})().(resourceGroupsLoadedMsg)
	if !reflect.DeepEqual(rgs.resourceGroups, client.resourceGroups) || client.resourceGroupSubscription != "sub" {
		t.Fatalf("resource groups message = %#v", rgs)
	}

	client.err = wantErr
	if msg := fetchSubscriptionsCmd(ctx, client)().(subscriptionsLoadedMsg); !errors.Is(msg.err, wantErr) {
		t.Fatalf("command error = %v", msg.err)
	}
}

func TestCreateExemptionCommand(t *testing.T) {
	client := &fakeAzureClient{createOutput: "created"}
	assignment := azure.PolicyAssignment{ID: "assignment"}
	msg := createExemptionCmd(context.Background(), client, "scope", "rg", "sub", assignment, map[string]bool{"z": true, "a": true}, "ticket", "users", "date")().(exemptionCreatedMsg)
	if msg.output != "created" || msg.err != nil {
		t.Fatalf("created message = %#v", msg)
	}
	want := createCall{scope: "scope", scopeName: "rg", subscriptionName: "sub", assignment: assignment, refs: []string{"a", "z"}, ticket: "ticket", users: "users", expiration: "date"}
	if !reflect.DeepEqual(client.created, want) {
		t.Fatalf("CreateExemption call = %#v, want %#v", client.created, want)
	}
}

type createCall struct {
	scope, scopeName, subscriptionName string
	assignment                         azure.PolicyAssignment
	refs                               []string
	ticket, users, expiration          string
}

type fakeAzureClient struct {
	subscriptions  []azure.Subscription
	assignments    []azure.PolicyAssignment
	definitions    []azure.PolicyDefinitionRef
	resourceGroups []azure.ResourceGroup
	createOutput   string
	err            error

	assignmentSubscription    string
	definitionAssignment      azure.PolicyAssignment
	resourceGroupSubscription string
	created                   createCall
}

func (f *fakeAzureClient) ListSubscriptions(context.Context) ([]azure.Subscription, error) {
	return f.subscriptions, f.err
}

func (f *fakeAzureClient) ListAssignments(_ context.Context, subscription string) ([]azure.PolicyAssignment, error) {
	f.assignmentSubscription = subscription
	return f.assignments, f.err
}

func (f *fakeAzureClient) ListAssignmentDefinitions(_ context.Context, assignment azure.PolicyAssignment) ([]azure.PolicyDefinitionRef, error) {
	f.definitionAssignment = assignment
	return f.definitions, f.err
}

func (f *fakeAzureClient) ListResourceGroups(_ context.Context, subscription string) ([]azure.ResourceGroup, error) {
	f.resourceGroupSubscription = subscription
	return f.resourceGroups, f.err
}

func (f *fakeAzureClient) CreateExemption(_ context.Context, scope, scopeName, subscriptionName string, assignment azure.PolicyAssignment, refs []string, ticket, users, expiration string) (string, error) {
	f.created = createCall{scope, scopeName, subscriptionName, assignment, refs, ticket, users, expiration}
	return f.createOutput, f.err
}
