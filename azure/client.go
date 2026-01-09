package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) EnsureLogin(ctx context.Context) error {
	if _, err := exec.LookPath("az"); err != nil {
		return fmt.Errorf("azure CLI (az) not found in PATH: %w", err)
	}
	if _, err := c.runAzCommand(ctx, "account", "show"); err == nil {
		return nil
	}

	fmt.Println("No active Azure CLI session detected. Launching 'az login'...")
	cmd := exec.CommandContext(ctx, "az", "login")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("az login failed: %w", err)
	}
	return nil
}

func (c *Client) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	data, err := c.runAzCommand(ctx, "account", "list", "--query", "[].{name:name,id:id}", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}
	var subs []Subscription
	if err := json.Unmarshal(data, &subs); err != nil {
		return nil, fmt.Errorf("unable to parse subscription data: %w", err)
	}
	sort.Slice(subs, func(i, j int) bool {
		return strings.ToLower(subs[i].Name) < strings.ToLower(subs[j].Name)
	})
	return subs, nil
}

func (c *Client) ListResourceGroups(ctx context.Context, subscriptionID string) ([]ResourceGroup, error) {
	data, err := c.runAzCommand(ctx, "group", "list", "--subscription", subscriptionID, "--query", "[].{name:name,id:id}", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list resource groups: %w", err)
	}
	var rgs []ResourceGroup
	if err := json.Unmarshal(data, &rgs); err != nil {
		return nil, fmt.Errorf("unable to parse resource group data: %w", err)
	}
	sort.Slice(rgs, func(i, j int) bool {
		return strings.ToLower(rgs[i].Name) < strings.ToLower(rgs[j].Name)
	})
	return rgs, nil
}

func (c *Client) ListAssignments(ctx context.Context, subscriptionID string) ([]PolicyAssignment, error) {
	var allAssignments []PolicyAssignment
	uri := fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/policyAssignments?api-version=2021-06-01", subscriptionID)

	for uri != "" {
		args := []string{
			"rest",
			"--method", "get",
			"--uri", uri,
			"--subscription", subscriptionID,
			"--query", "{value:value[].{id:id,name:name,displayName:properties.displayName,scope:properties.scope,policyDefinitionId:properties.policyDefinitionId},nextLink:nextLink}",
			"-o", "json",
		}
		data, err := c.runAzCommand(ctx, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to list policy assignments: %w", err)
		}

		var result struct {
			Value    []PolicyAssignment `json:"value"`
			NextLink string             `json:"nextLink"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("unable to parse assignment data: %w", err)
		}

		allAssignments = append(allAssignments, result.Value...)
		uri = result.NextLink
	}

	sort.Slice(allAssignments, func(i, j int) bool {
		return strings.ToLower(allAssignments[i].DisplayLabel()) < strings.ToLower(allAssignments[j].DisplayLabel())
	})
	return allAssignments, nil
}

func (c *Client) ListAssignmentDefinitions(ctx context.Context, assignment PolicyAssignment) ([]PolicyDefinitionRef, error) {
	if assignment.PolicyDefinitionID == "" {
		return nil, nil
	}
	if !strings.Contains(strings.ToLower(assignment.PolicyDefinitionID), "policysetdefinitions") {
		return nil, nil
	}

	name, sub, mg := c.parsePolicyID(assignment.PolicyDefinitionID)
	if name == "" {
		return nil, fmt.Errorf("could not parse policy set name from ID: %s", assignment.PolicyDefinitionID)
	}

	args := []string{
		"policy", "set-definition", "show",
		"--name", name,
	}
	if mg != "" {
		args = append(args, "--management-group", mg)
	} else if sub != "" {
		args = append(args, "--subscription", sub)
	}
	args = append(args, "--query", "{policyDefinitions:policyDefinitions}", "-o", "json")

	data, err := c.runAzCommand(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load policy set definition (ID: '%s'): %w", assignment.PolicyDefinitionID, err)
	}
	var set struct {
		PolicyDefinitions []struct {
			PolicyDefinitionID string `json:"policyDefinitionId"`
			ReferenceID        string `json:"policyDefinitionReferenceId"`
		} `json:"policyDefinitions"`
	}
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("unable to parse policy set definition: %w", err)
	}
	var refs []PolicyDefinitionRef
	for _, def := range set.PolicyDefinitions {
		display := def.PolicyDefinitionID
		if name, err := c.policyDisplayName(ctx, def.PolicyDefinitionID); err == nil && name != "" {
			display = name
		}
		refs = append(refs, PolicyDefinitionRef{
			PolicyDefinitionID: def.PolicyDefinitionID,
			ReferenceID:        def.ReferenceID,
			DisplayName:        display,
		})
	}
	sort.Slice(refs, func(i, j int) bool {
		return strings.ToLower(refs[i].DisplayName) < strings.ToLower(refs[j].DisplayName)
	})
	return refs, nil
}

func (c *Client) CreateExemption(ctx context.Context, scope string, assignment PolicyAssignment, referenceIDs []string, ticket, users, expirationDate string) (string, error) {
	description := fmt.Sprintf("Ticket %s raised by %s on %s", ticket, users, time.Now().Format(time.RFC3339))
	args := []string{
		"policy", "exemption", "create",
		"--name", ticket,
		"--scope", scope,
		"--policy-assignment", assignment.ID,
		"--display-name", fmt.Sprintf("%s/%s %s", scope, assignment.DisplayName, ticket),
		"--description", description,
		"--exemption-category", "Waiver",
		"-o", "json",
	}
	if expirationDate != "" {
		t, _ := time.Parse("2006-01-02", expirationDate)
		t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		args = append(args, "--expires-on", t.Format(time.RFC3339))
	}
	if len(referenceIDs) > 0 {
		args = append(args, "--policy-definition-reference-ids")
		args = append(args, referenceIDs...)
	}
	data, err := c.runAzCommand(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("failed to create policy exemption: %w", err)
	}
	return string(data), nil
}

func (c *Client) policyDisplayName(ctx context.Context, definitionID string) (string, error) {
	if definitionID == "" {
		return "", nil
	}

	name, sub, mg := c.parsePolicyID(definitionID)
	if name == "" {
		return "", fmt.Errorf("could not parse policy definition name from ID: %s", definitionID)
	}

	args := []string{
		"policy", "definition", "show",
		"--name", name,
	}
	if mg != "" {
		args = append(args, "--management-group", mg)
	} else if sub != "" {
		args = append(args, "--subscription", sub)
	}
	args = append(args, "--query", "{displayName:displayName,name:name}", "-o", "json")

	data, err := c.runAzCommand(ctx, args...)
	if err != nil {
		return "", err
	}
	var def struct {
		DisplayName string `json:"displayName"`
		Name        string `json:"name"`
	}
	if err := json.Unmarshal(data, &def); err != nil {
		return "", err
	}
	if def.DisplayName != "" {
		return def.DisplayName, nil
	}
	return def.Name, nil
}

func (c *Client) parsePolicyID(id string) (name, subscription, managementGroup string) {
	parts := strings.Split(id, "/")
	for i, part := range parts {
		if strings.EqualFold(part, "subscriptions") && i+1 < len(parts) {
			subscription = parts[i+1]
		}
		if strings.EqualFold(part, "managementGroups") && i+1 < len(parts) {
			managementGroup = parts[i+1]
		}
		if (strings.EqualFold(part, "policySetDefinitions") || strings.EqualFold(part, "policyDefinitions")) && i+1 < len(parts) {
			name = parts[i+1]
		}
	}
	return
}

func (c *Client) runAzCommand(ctx context.Context, args ...string) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "az", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}
