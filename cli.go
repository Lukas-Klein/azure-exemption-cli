package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
)

type step int

const (
	stepLoadingSubscriptions step = iota
	stepSelectSubscription
	stepLoadingAssignments
	stepSelectAssignment
	stepLoadingAssignmentDefinitions
	stepAssignmentScope
	stepSelectDefinitions
	stepLoadingResourceGroups
	stepSelectResourceGroup
	stepTicket
	stepUsers
	stepConfirm
	stepCreating
	stepDone
	stepError
)

type subscription struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type resourceGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type policyAssignment struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	DisplayName        string `json:"displayName"`
	Scope              string `json:"scope"`
	PolicyDefinitionID string `json:"policyDefinitionId"`
}

type policyDefinitionRef struct {
	PolicyDefinitionID string `json:"policyDefinitionId"`
	ReferenceID        string `json:"policyDefinitionReferenceId"`
	DisplayName        string
}

const maxVisibleSubscriptions = 15

func (s subscription) scope() string {
	if strings.HasPrefix(s.ID, "/") {
		return s.ID
	}
	return "/subscriptions/" + s.ID
}

func (s subscription) shortID() string {
	if !strings.HasPrefix(s.ID, "/subscriptions/") {
		return s.ID
	}
	parts := strings.Split(s.ID, "/")
	return parts[len(parts)-1]
}

func (p policyAssignment) displayLabel() string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	return p.Name
}

func (p policyAssignment) shortID() string {
	if p.ID == "" {
		return ""
	}
	parts := strings.Split(p.ID, "/")
	return parts[len(parts)-1]
}

type subscriptionsLoadedMsg struct {
	subscriptions []subscription
	err           error
}

type assignmentsLoadedMsg struct {
	assignments []policyAssignment
	err         error
}

type assignmentDefinitionsLoadedMsg struct {
	definitions []policyDefinitionRef
	err         error
}

type resourceGroupsLoadedMsg struct {
	resourceGroups []resourceGroup
	err            error
}

type exemptionCreatedMsg struct {
	output string
	err    error
}

type model struct {
	ctx context.Context

	step   step
	status string
	err    error

	subscriptions         []subscription
	assignments           []policyAssignment
	assignmentDefinitions []policyDefinitionRef
	resourceGroups        []resourceGroup
	selectedDefinitionIDs map[string]bool
	cursor                int
	selectedSubscription  int
	selectedAssignment    int
	selectedResourceGroup int
	partialExemption      bool

	ticketInput textinput.Model
	userInput   textinput.Model

	ticket      string
	requestUser string

	createOutput string
}

func newModel(ctx context.Context) *model {
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

	return &model{
		ctx:                   ctx,
		step:                  stepLoadingSubscriptions,
		selectedSubscription:  -1,
		selectedAssignment:    -1,
		selectedResourceGroup: -1,
		selectedDefinitionIDs: make(map[string]bool),
		ticketInput:           ticketInput,
		userInput:             userInput,
	}
}

func (m *model) Init() tea.Cmd {
	return fetchSubscriptionsCmd(m.ctx)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		return m, m.handleKey(msg)

	case subscriptionsLoadedMsg:
		if msg.err != nil {
			return m.fail(msg.err)
		}
		if len(msg.subscriptions) == 0 {
			return m.fail(errors.New("no subscriptions returned by Azure CLI"))
		}
		m.subscriptions = msg.subscriptions
		m.cursor = 0
		m.selectedSubscription = -1
		m.step = stepSelectSubscription
		m.status = "Use ↑/↓ to highlight a subscription and press Enter to continue."
		return m, nil

	case assignmentsLoadedMsg:
		if msg.err != nil {
			return m.fail(msg.err)
		}
		if len(msg.assignments) == 0 {
			sub := m.currentSubscription()
			return m.fail(fmt.Errorf("no policy assignments were returned for subscription %s (%s)", sub.Name, sub.shortID()))
		}
		m.assignments = msg.assignments
		m.selectedAssignment = -1
		m.assignmentDefinitions = nil
		m.selectedDefinitionIDs = make(map[string]bool)
		m.partialExemption = false
		m.cursor = 0
		m.step = stepSelectAssignment
		m.status = "Use ↑/↓ to highlight an assignment and press Enter to continue."
		return m, nil

	case assignmentDefinitionsLoadedMsg:
		if msg.err != nil {
			return m.fail(msg.err)
		}
		m.assignmentDefinitions = msg.definitions
		m.selectedDefinitionIDs = make(map[string]bool)
		m.partialExemption = false
		if len(msg.definitions) > 1 {
			m.step = stepAssignmentScope
			m.cursor = 0
			m.status = "Exempt entire assignment or select specific definitions?"
		} else {
			m.partialExemption = false
			m.step = stepLoadingResourceGroups
			m.status = "Loading resource groups..."
			return m, fetchResourceGroupsCmd(m.ctx, m.currentSubscription())
		}
		return m, nil

	case resourceGroupsLoadedMsg:
		if msg.err != nil {
			return m.fail(msg.err)
		}
		// Prepend "Entire Subscription" option
		sub := m.currentSubscription()
		entireSub := resourceGroup{
			Name: "Entire Subscription",
			ID:   sub.scope(),
		}
		m.resourceGroups = append([]resourceGroup{entireSub}, msg.resourceGroups...)
		m.selectedResourceGroup = -1
		m.cursor = 0
		m.step = stepSelectResourceGroup
		m.status = "Select the scope for the exemption (Subscription or Resource Group)."
		return m, nil

	case exemptionCreatedMsg:
		if msg.err != nil {
			return m.fail(msg.err)
		}
		m.createOutput = msg.output
		m.step = stepDone
		m.status = "Exemption created successfully. Press q to exit."
		return m, nil
	}

	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch m.step {
	case stepSelectSubscription:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.subscriptions)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.subscriptions) == 0 {
				return nil
			}
			m.selectedSubscription = m.cursor
			m.step = stepLoadingAssignments
			sub := m.currentSubscription()
			m.status = fmt.Sprintf("Fetching policy assignments for %s...", sub.Name)
			return fetchAssignmentsCmd(m.ctx, sub)
		}

	case stepSelectAssignment:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.assignments)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.assignments) == 0 {
				return nil
			}
			m.selectedAssignment = m.cursor
			m.step = stepLoadingAssignmentDefinitions
			assign := m.currentAssignment()
			m.status = fmt.Sprintf("Fetching assignment details for %s...", assign.displayLabel())
			return fetchAssignmentDefinitionsCmd(m.ctx, assign)
		}

	case stepAssignmentScope:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < 1 {
				m.cursor++
			}
		case "enter":
			if m.cursor == 0 {
				m.partialExemption = false
				m.step = stepLoadingResourceGroups
				m.status = "Loading resource groups..."
				return fetchResourceGroupsCmd(m.ctx, m.currentSubscription())
			}
			m.partialExemption = true
			m.step = stepSelectDefinitions
			m.cursor = 0
			m.status = "Select definitions to exempt (space to toggle, Enter to continue)."
			return nil
		}

	case stepSelectDefinitions:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.assignmentDefinitions)-1 {
				m.cursor++
			}
		case " ":
			if len(m.assignmentDefinitions) == 0 {
				return nil
			}
			ref := m.assignmentDefinitions[m.cursor]
			if m.selectedDefinitionIDs[ref.ReferenceID] {
				delete(m.selectedDefinitionIDs, ref.ReferenceID)
			} else {
				m.selectedDefinitionIDs[ref.ReferenceID] = true
			}
		case "enter":
			if len(m.assignmentDefinitions) == 0 {
				return nil
			}
			if len(m.selectedDefinitionIDs) == 0 {
				m.status = "Select at least one definition or choose full assignment."
				return nil
			}
			m.step = stepLoadingResourceGroups
			m.status = "Loading resource groups..."
			return fetchResourceGroupsCmd(m.ctx, m.currentSubscription())
		}

	case stepSelectResourceGroup:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.resourceGroups)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.resourceGroups) == 0 {
				return nil
			}
			m.selectedResourceGroup = m.cursor
			m.step = stepTicket
			m.ticketInput.SetValue("")
			m.ticketInput.Focus()
			m.status = "Provide the tracking ticket number linked to this exemption:"
			return nil
		}

	case stepTicket:
		var textCmd tea.Cmd
		m.ticketInput, textCmd = m.ticketInput.Update(msg)
		if msg.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.ticketInput.Value())
			if value == "" {
				m.status = "A ticket number is required."
				return textCmd
			}
			m.ticket = value
			m.step = stepUsers
			m.ticketInput.Blur()
			m.userInput.SetValue("")
			m.userInput.Focus()
			m.status = "Who is requesting this exemption? Provide one or more names."
			return textCmd
		}
		return textCmd

	case stepUsers:
		var textCmd tea.Cmd
		m.userInput, textCmd = m.userInput.Update(msg)
		if msg.Type == tea.KeyEnter {
			value := strings.TrimSpace(m.userInput.Value())
			if value == "" {
				m.status = "At least one requester name is required."
				return textCmd
			}
			m.requestUser = value
			m.step = stepConfirm
			m.userInput.Blur()
			m.status = "Review the summary and press Enter to create the exemption."
			return textCmd
		}
		return textCmd

	case stepConfirm:
		if msg.Type == tea.KeyEnter {
			if m.selectedAssignment < 0 || m.ticket == "" || m.requestUser == "" || m.selectedSubscription < 0 || m.selectedResourceGroup < 0 {
				m.status = "Missing information. Use q to abort."
				return nil
			}
			m.step = stepCreating
			assign := m.currentAssignment()
			rg := m.resourceGroups[m.selectedResourceGroup]
			m.status = "Creating Azure Policy exemption..."
			return createExemptionCmd(m.ctx, rg.ID, assign, m.selectedDefinitionIDs, m.ticket, m.requestUser)
		}

	case stepError, stepDone, stepLoadingAssignmentDefinitions, stepLoadingAssignments, stepLoadingSubscriptions, stepLoadingResourceGroups, stepCreating:
		// No interactive keys beyond quit for these states.
	}

	return nil
}

func (m *model) View() string {
	var b strings.Builder
	b.WriteString("Azure Policy Exemption CLI\n\n")

	switch m.step {
	case stepLoadingSubscriptions:
		b.WriteString("Retrieving subscriptions via Azure CLI...\n")

	case stepSelectSubscription:
		b.WriteString("Select the subscription for the exemption:\n\n")
		start, end := visibleRange(m.cursor, len(m.subscriptions), maxVisibleSubscriptions)
		for i := start; i < end; i++ {
			sub := m.subscriptions[i]
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			marker := " "
			if i == m.selectedSubscription {
				marker = "x"
			}
			line := fmt.Sprintf("%s [%s] %s (%s)", cursor, marker, sub.Name, sub.shortID())
			if i == m.cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		fmt.Fprintf(&b, "\nShowing %d-%d of %d\n", start+1, end, len(m.subscriptions))
		b.WriteString("↑/↓ to move, Enter to select.\n")

	case stepLoadingAssignments:
		b.WriteString("Loading policy assignments for the selected subscription...\n")

	case stepSelectAssignment:
		sub := m.currentSubscription()
		fmt.Fprintf(&b, "Policy assignments for subscription %s (%s):\n\n", sub.Name, sub.shortID())
		start, end := visibleRange(m.cursor, len(m.assignments), maxVisibleSubscriptions)
		for i := start; i < end; i++ {
			assign := m.assignments[i]
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			marker := " "
			if i == m.selectedAssignment {
				marker = "x"
			}
			line := fmt.Sprintf("%s [%s] %s (%s)", cursor, marker, assign.displayLabel(), assign.shortID())
			if i == m.cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		fmt.Fprintf(&b, "\nShowing %d-%d of %d\n", start+1, end, len(m.assignments))
		b.WriteString("↑/↓ to move, Enter to select.\n")

	case stepLoadingAssignmentDefinitions:
		b.WriteString("Loading assignment details...\n")

	case stepAssignmentScope:
		b.WriteString("This assignment contains multiple policy definitions.\n\n")
		options := []string{"Exempt entire assignment", "Exempt specific definitions"}
		for i, opt := range options {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			line := fmt.Sprintf("%s %s", cursor, opt)
			if i == m.cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		b.WriteString("\n↑/↓ to move, Enter to choose.\n")

	case stepSelectDefinitions:
		b.WriteString("Select the policy definitions to exempt:\n\n")
		start, end := visibleRange(m.cursor, len(m.assignmentDefinitions), maxVisibleSubscriptions)
		for i := start; i < end; i++ {
			ref := m.assignmentDefinitions[i]
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			marker := " "
			if m.selectedDefinitionIDs[ref.ReferenceID] {
				marker = "x"
			}
			line := fmt.Sprintf("%s [%s] %s (%s)", cursor, marker, ref.DisplayName, ref.ReferenceID)
			if i == m.cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		fmt.Fprintf(&b, "\nShowing %d-%d of %d\n", start+1, end, len(m.assignmentDefinitions))
		b.WriteString("↑/↓ to move, Space to toggle, Enter to continue.\n")

	case stepLoadingResourceGroups:
		b.WriteString("Loading resource groups...\n")

	case stepSelectResourceGroup:
		b.WriteString("Select the scope for the exemption:\n\n")
		start, end := visibleRange(m.cursor, len(m.resourceGroups), maxVisibleSubscriptions)
		for i := start; i < end; i++ {
			rg := m.resourceGroups[i]
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			marker := " "
			if i == m.selectedResourceGroup {
				marker = "x"
			}
			line := fmt.Sprintf("%s [%s] %s", cursor, marker, rg.Name)
			if i == m.cursor {
				line = selectedStyle.Render(line)
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
		fmt.Fprintf(&b, "\nShowing %d-%d of %d\n", start+1, end, len(m.resourceGroups))
		b.WriteString("↑/↓ to move, Enter to select.\n")

	case stepTicket:
		assign := m.currentAssignment()
		fmt.Fprintf(&b, "Assignment selected: %s\n\n", assign.displayLabel())
		if m.partialExemption && len(m.selectedDefinitionIDs) > 0 {
			b.WriteString("Definitions selected:\n")
			for _, ref := range m.assignmentDefinitions {
				if m.selectedDefinitionIDs[ref.ReferenceID] {
					fmt.Fprintf(&b, "• %s (%s)\n", ref.DisplayName, ref.ReferenceID)
				}
			}
			b.WriteString("\n")
		}
		b.WriteString("Provide the tracking ticket number linked to this exemption:\n\n")
		b.WriteString(m.ticketInput.View() + "\n")

	case stepUsers:
		assign := m.currentAssignment()
		fmt.Fprintf(&b, "Ticket: %s\nAssignment: %s\n\n", m.ticket, assign.displayLabel())
		b.WriteString("Who is requesting this exemption? (comma separated)\n\n")
		b.WriteString(m.userInput.View() + "\n")

	case stepConfirm:
		sub := m.currentSubscription()
		assign := m.currentAssignment()
		rg := m.resourceGroups[m.selectedResourceGroup]
		fmt.Fprintf(&b, "Subscription: %s (%s)\n", sub.Name, sub.shortID())
		fmt.Fprintf(&b, "Scope: %s\n", rg.Name)
		fmt.Fprintf(&b, "Assignment: %s\n", assign.displayLabel())
		if m.partialExemption && len(m.selectedDefinitionIDs) > 0 {
			b.WriteString("Definitions:\n")
			for _, ref := range m.assignmentDefinitions {
				if m.selectedDefinitionIDs[ref.ReferenceID] {
					fmt.Fprintf(&b, "  %s (%s)\n", ref.DisplayName, ref.ReferenceID)
				}
			}
		} else {
			b.WriteString("Definitions: Entire assignment\n")
		}
		fmt.Fprintf(&b, "Ticket: %s\n", m.ticket)
		fmt.Fprintf(&b, "Requesters: %s\n\n", m.requestUser)
		b.WriteString("Press Enter to create the exemption or q to abort.\n")

	case stepCreating:
		b.WriteString("Creating policy exemption via Azure CLI...\n")

	case stepDone:
		b.WriteString("Azure CLI response:\n\n")
		if m.createOutput == "" {
			b.WriteString("No output returned.\n")
		} else {
			b.WriteString(m.createOutput + "\n")
		}
		b.WriteString("\nPress q to exit.\n")

	case stepError:
		fmt.Fprintf(&b, "Error: %v\n\nPress q to exit.\n", m.err)
	}

	if m.status != "" {
		b.WriteString("\n" + m.status + "\n")
	}

	return b.String()
}

func (m *model) currentSubscription() subscription {
	if m.selectedSubscription >= 0 && m.selectedSubscription < len(m.subscriptions) {
		return m.subscriptions[m.selectedSubscription]
	}
	if len(m.subscriptions) == 0 {
		return subscription{}
	}
	return m.subscriptions[0]
}

func (m *model) currentAssignment() policyAssignment {
	if m.selectedAssignment >= 0 && m.selectedAssignment < len(m.assignments) {
		return m.assignments[m.selectedAssignment]
	}
	if len(m.assignments) == 0 {
		return policyAssignment{}
	}
	return m.assignments[0]
}

func (m *model) fail(err error) (tea.Model, tea.Cmd) {
	m.err = err
	m.step = stepError
	m.status = ""
	return m, nil
}

func visibleRange(cursor, total, limit int) (start, end int) {
	if limit <= 0 || total <= limit {
		return 0, total
	}
	start = cursor - limit/2
	if start < 0 {
		start = 0
	}
	end = start + limit
	if end > total {
		end = total
		start = end - limit
	}
	return start, end
}

func ensureAzureLogin(ctx context.Context) error {
	if _, err := exec.LookPath("az"); err != nil {
		return fmt.Errorf("azure CLI (az) not found in PATH: %w", err)
	}
	if _, err := runAzCommand(ctx, "account", "show"); err == nil {
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

func fetchSubscriptionsCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		subs, err := listSubscriptions(ctx)
		return subscriptionsLoadedMsg{subscriptions: subs, err: err}
	}
}

func fetchAssignmentsCmd(ctx context.Context, sub subscription) tea.Cmd {
	return func() tea.Msg {
		assignments, err := listAssignments(ctx, sub.shortID())
		return assignmentsLoadedMsg{assignments: assignments, err: err}
	}
}

func fetchAssignmentDefinitionsCmd(ctx context.Context, assignment policyAssignment) tea.Cmd {
	return func() tea.Msg {
		definitions, err := listAssignmentDefinitions(ctx, assignment)
		return assignmentDefinitionsLoadedMsg{definitions: definitions, err: err}
	}
}

func fetchResourceGroupsCmd(ctx context.Context, sub subscription) tea.Cmd {
	return func() tea.Msg {
		rgs, err := listResourceGroups(ctx, sub.shortID())
		return resourceGroupsLoadedMsg{resourceGroups: rgs, err: err}
	}
}

func createExemptionCmd(ctx context.Context, scope string, assignment policyAssignment, selectedDefinitionIDs map[string]bool, ticket, users string) tea.Cmd {
	return func() tea.Msg {
		var refs []string
		for ref := range selectedDefinitionIDs {
			refs = append(refs, ref)
		}
		output, err := createExemption(ctx, scope, assignment, refs, ticket, users)
		return exemptionCreatedMsg{output: output, err: err}
	}
}

func listSubscriptions(ctx context.Context) ([]subscription, error) {
	data, err := runAzCommand(ctx, "account", "list", "--query", "[].{name:name,id:id}", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}
	var subs []subscription
	if err := json.Unmarshal(data, &subs); err != nil {
		return nil, fmt.Errorf("unable to parse subscription data: %w", err)
	}
	sort.Slice(subs, func(i, j int) bool {
		return strings.ToLower(subs[i].Name) < strings.ToLower(subs[j].Name)
	})
	return subs, nil
}

func listAssignments(ctx context.Context, subscriptionID string) ([]policyAssignment, error) {
	var allAssignments []policyAssignment
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
		data, err := runAzCommand(ctx, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to list policy assignments: %w", err)
		}

		var result struct {
			Value    []policyAssignment `json:"value"`
			NextLink string             `json:"nextLink"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("unable to parse assignment data: %w", err)
		}

		allAssignments = append(allAssignments, result.Value...)
		uri = result.NextLink
	}

	sort.Slice(allAssignments, func(i, j int) bool {
		return strings.ToLower(allAssignments[i].displayLabel()) < strings.ToLower(allAssignments[j].displayLabel())
	})
	return allAssignments, nil
}

func listResourceGroups(ctx context.Context, subscriptionID string) ([]resourceGroup, error) {
	data, err := runAzCommand(ctx, "group", "list", "--subscription", subscriptionID, "--query", "[].{name:name,id:id}", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list resource groups: %w", err)
	}
	var rgs []resourceGroup
	if err := json.Unmarshal(data, &rgs); err != nil {
		return nil, fmt.Errorf("unable to parse resource group data: %w", err)
	}
	sort.Slice(rgs, func(i, j int) bool {
		return strings.ToLower(rgs[i].Name) < strings.ToLower(rgs[j].Name)
	})
	return rgs, nil
}

func listAssignmentDefinitions(ctx context.Context, assignment policyAssignment) ([]policyDefinitionRef, error) {
	if assignment.PolicyDefinitionID == "" {
		return nil, nil
	}
	if !strings.Contains(strings.ToLower(assignment.PolicyDefinitionID), "policysetdefinitions") {
		return nil, nil
	}

	name, sub, mg := parsePolicyID(assignment.PolicyDefinitionID)
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

	data, err := runAzCommand(ctx, args...)
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
	var refs []policyDefinitionRef
	for _, def := range set.PolicyDefinitions {
		display := def.PolicyDefinitionID
		if name, err := policyDisplayName(ctx, def.PolicyDefinitionID); err == nil && name != "" {
			display = name
		}
		refs = append(refs, policyDefinitionRef{
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

func policyDisplayName(ctx context.Context, definitionID string) (string, error) {
	if definitionID == "" {
		return "", nil
	}

	name, sub, mg := parsePolicyID(definitionID)
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

	data, err := runAzCommand(ctx, args...)
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

func createExemption(ctx context.Context, scope string, assignment policyAssignment, referenceIDs []string, ticket, users string) (string, error) {
	description := fmt.Sprintf("Ticket %s raised by %s on %s", ticket, users, time.Now().Format(time.RFC3339))
	args := []string{
		"policy", "exemption", "create",
		"--name", ticket,
		"--scope", scope,
		"--policy-assignment", assignment.ID,
		"--display-name", fmt.Sprintf("%s exemption", ticket),
		"--description", description,
		"--exemption-category", "Waiver",
		"-o", "json",
	}
	if len(referenceIDs) > 0 {
		args = append(args, "--policy-definition-reference-ids")
		args = append(args, referenceIDs...)
	}
	data, err := runAzCommand(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("failed to create policy exemption: %w", err)
	}
	return string(data), nil
}

func parsePolicyID(id string) (name, subscription, managementGroup string) {
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

func runAzCommand(ctx context.Context, args ...string) ([]byte, error) {
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

func main() {
	ctx := context.Background()
	if err := ensureAzureLogin(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Azure login failed: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(newModel(ctx))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
