// Package azure is for interacting with Azure resources
package azure

import "strings"

type Subscription struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s Subscription) Scope() string {
	if strings.HasPrefix(s.ID, "/") {
		return s.ID
	}
	return "/subscriptions/" + s.ID
}

func (s Subscription) ShortID() string {
	if !strings.HasPrefix(s.ID, "/subscriptions/") {
		return s.ID
	}
	parts := strings.Split(s.ID, "/")
	return parts[len(parts)-1]
}

type ResourceGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PolicyAssignment struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	DisplayName        string `json:"displayName"`
	Scope              string `json:"scope"`
	PolicyDefinitionID string `json:"policyDefinitionId"`
}

func (p PolicyAssignment) DisplayLabel() string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	return p.Name
}

func (p PolicyAssignment) ShortID() string {
	if p.ID == "" {
		return ""
	}
	parts := strings.Split(p.ID, "/")
	return parts[len(parts)-1]
}

type PolicyDefinitionRef struct {
	PolicyDefinitionID string `json:"policyDefinitionId"`
	ReferenceID        string `json:"policyDefinitionReferenceId"`
	DisplayName        string
}
