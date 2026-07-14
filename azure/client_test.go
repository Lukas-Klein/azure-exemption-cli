package azure

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestListSubscriptionsAndResourceGroups(t *testing.T) {
	log := installFakeAz(t)
	t.Setenv("AZ_ACCOUNT_LIST", `[{"name":"zeta","id":"2"},{"name":"Alpha","id":"1"}]`)
	t.Setenv("AZ_GROUP_LIST", `[{"name":"west","id":"/west"},{"name":"East","id":"/east"}]`)

	c := NewClient()
	subs, err := c.ListSubscriptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{subs[0].Name, subs[1].Name}; !reflect.DeepEqual(got, []string{"Alpha", "zeta"}) {
		t.Fatalf("subscriptions = %#v", got)
	}
	rgs, err := c.ListResourceGroups(context.Background(), "sub-1")
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{rgs[0].Name, rgs[1].Name}; !reflect.DeepEqual(got, []string{"East", "west"}) {
		t.Fatalf("resource groups = %#v", got)
	}
	assertLogContains(t, log, "account list --query [].{name:name,id:id} -o json")
	assertLogContains(t, log, "group list --subscription sub-1 --query [].{name:name,id:id} -o json")
}

func TestListCommandErrors(t *testing.T) {
	installFakeAz(t)
	t.Setenv("AZ_FAIL_MATCH", "account list")
	t.Setenv("AZ_FAIL_MESSAGE", "not authorized")
	if _, err := NewClient().ListSubscriptions(context.Background()); err == nil || !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("ListSubscriptions() error = %v", err)
	}

	t.Setenv("AZ_FAIL_MATCH", "")
	t.Setenv("AZ_ACCOUNT_LIST", "not-json")
	if _, err := NewClient().ListSubscriptions(context.Background()); err == nil || !strings.Contains(err.Error(), "parse subscription") {
		t.Fatalf("ListSubscriptions() parse error = %v", err)
	}

	t.Setenv("AZ_GROUP_LIST", "not-json")
	if _, err := NewClient().ListResourceGroups(context.Background(), "sub"); err == nil || !strings.Contains(err.Error(), "parse resource group") {
		t.Fatalf("ListResourceGroups() parse error = %v", err)
	}
}

func TestListAssignmentsPaginationAndSorting(t *testing.T) {
	log := installFakeAz(t)
	t.Setenv("AZ_REST_FIRST", `{"value":[{"id":"/a/z","name":"z","displayName":"Zulu"}],"nextLink":"https://next/page"}`)
	t.Setenv("AZ_REST_NEXT", `{"value":[{"id":"/a/a","name":"a","displayName":"alpha"}],"nextLink":""}`)

	got, err := NewClient().ListAssignments(context.Background(), "sub-1")
	if err != nil {
		t.Fatal(err)
	}
	if labels := []string{got[0].DisplayLabel(), got[1].DisplayLabel()}; !reflect.DeepEqual(labels, []string{"alpha", "Zulu"}) {
		t.Fatalf("assignment labels = %#v", labels)
	}
	assertLogContains(t, log, "--uri /subscriptions/sub-1/providers/Microsoft.Authorization/policyAssignments?api-version=2021-06-01")
	assertLogContains(t, log, "--uri https://next/page")

	t.Setenv("AZ_REST_FIRST", "bad-json")
	if _, err := NewClient().ListAssignments(context.Background(), "sub-1"); err == nil || !strings.Contains(err.Error(), "parse assignment") {
		t.Fatalf("ListAssignments() parse error = %v", err)
	}
}

func TestListAssignmentDefinitions(t *testing.T) {
	log := installFakeAz(t)
	c := NewClient()
	if got, err := c.ListAssignmentDefinitions(context.Background(), PolicyAssignment{}); err != nil || got != nil {
		t.Fatalf("empty definition ID = %#v, %v", got, err)
	}
	if got, err := c.ListAssignmentDefinitions(context.Background(), PolicyAssignment{PolicyDefinitionID: "/policyDefinitions/a"}); err != nil || got != nil {
		t.Fatalf("single definition ID = %#v, %v", got, err)
	}

	t.Setenv("AZ_SET_SHOW", `{"policyDefinitions":[{"policyDefinitionId":"/subscriptions/s/providers/Microsoft.Authorization/policyDefinitions/z","policyDefinitionReferenceId":"ref-z"},{"policyDefinitionId":"/providers/Microsoft.Authorization/policyDefinitions/a","policyDefinitionReferenceId":"ref-a"}]}`)
	t.Setenv("AZ_DEF_Z", `{"displayName":"Zulu","name":"z"}`)
	t.Setenv("AZ_DEF_A", `{"displayName":"Alpha","name":"a"}`)
	assignment := PolicyAssignment{PolicyDefinitionID: "/subscriptions/s/providers/Microsoft.Authorization/policySetDefinitions/set1"}
	refs, err := c.ListAssignmentDefinitions(context.Background(), assignment)
	if err != nil {
		t.Fatal(err)
	}
	if names := []string{refs[0].DisplayName, refs[1].DisplayName}; !reflect.DeepEqual(names, []string{"Alpha", "Zulu"}) {
		t.Fatalf("definition names = %#v", names)
	}
	assertLogContains(t, log, "policy set-definition show --name set1 --subscription s")
	assertLogContains(t, log, "policy definition show --name a")

	t.Setenv("AZ_SET_SHOW", "bad-json")
	if _, err := c.ListAssignmentDefinitions(context.Background(), assignment); err == nil || !strings.Contains(err.Error(), "parse policy set") {
		t.Fatalf("parse error = %v", err)
	}
}

func TestCreateExemptionArguments(t *testing.T) {
	log := installFakeAz(t)
	t.Setenv("AZ_CREATE", `{"name":"created"}`)
	assignment := PolicyAssignment{ID: "/assignments/a", DisplayName: "Require TLS"}
	out, err := NewClient().CreateExemption(context.Background(), "/subscriptions/s/resourceGroups/rg", "rg", "Production", assignment, []string{"ref-a", "ref-b"}, "INC123", "Ada", "2030-05-06")
	if err != nil || out != `{"name":"created"}` {
		t.Fatalf("CreateExemption() = %q, %v", out, err)
	}
	assertLogContains(t, log, "policy exemption create --name Production-rg---Require-TLS")
	assertLogContains(t, log, "--scope /subscriptions/s/resourceGroups/rg")
	assertLogContains(t, log, "--policy-assignment /assignments/a")
	assertLogContains(t, log, "--exemption-category Waiver")
	assertLogContains(t, log, "--expires-on 2030-05-06T23:59:59Z")
	assertLogContains(t, log, "--policy-definition-reference-ids ref-a ref-b")

	t.Setenv("AZ_FAIL_MATCH", "policy exemption create")
	if _, err := NewClient().CreateExemption(context.Background(), "/s", "Entire Subscription", "Prod", assignment, nil, "T", "U", ""); err == nil || !strings.Contains(err.Error(), "failed to create") {
		t.Fatalf("CreateExemption() error = %v", err)
	}
}

func TestSanitizeExemptionName(t *testing.T) {
	tests := map[string]string{
		"allowed-A_1.txt":       "allowed-A_1.txt",
		"scope / policy!":       "scope---policy",
		"é漢":                    "",
		strings.Repeat("a", 65): strings.Repeat("a", 64),
	}
	for input, want := range tests {
		if got := sanitizeExemptionName(input); got != want {
			t.Errorf("sanitizeExemptionName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParsePolicyID(t *testing.T) {
	tests := []struct {
		id, name, sub, mg string
	}{
		{"/subscriptions/s/providers/Microsoft.Authorization/policyDefinitions/p", "p", "s", ""},
		{"/providers/Microsoft.Management/managementGroups/m/providers/Microsoft.Authorization/policySetDefinitions/set", "set", "", "m"},
		{"/providers/Microsoft.Authorization/policyDefinitions/builtin", "builtin", "", ""},
		{"/SUBSCRIPTIONS/S/providers/x/POLICYDEFINITIONS/P", "P", "S", ""},
		{"/policyDefinitions", "", "", ""},
	}
	for _, tt := range tests {
		name, sub, mg := NewClient().parsePolicyID(tt.id)
		if name != tt.name || sub != tt.sub || mg != tt.mg {
			t.Errorf("parsePolicyID(%q) = %q, %q, %q", tt.id, name, sub, mg)
		}
	}
}

func TestEnsureLogin(t *testing.T) {
	installFakeAz(t)
	t.Setenv("AZ_ACCOUNT_SHOW", `{}`)
	if err := NewClient().EnsureLogin(context.Background()); err != nil {
		t.Fatalf("active login: %v", err)
	}

	t.Setenv("AZ_FAIL_MATCH", "account show")
	if err := NewClient().EnsureLogin(context.Background()); err != nil {
		t.Fatalf("login fallback: %v", err)
	}

	t.Setenv("AZ_LOGIN_FAIL", "1")
	if err := NewClient().EnsureLogin(context.Background()); err == nil || !strings.Contains(err.Error(), "az login failed") {
		t.Fatalf("failed login error = %v", err)
	}
}

func installFakeAz(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "az.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$AZ_TEST_LOG"
case "$*" in
  *"$AZ_FAIL_MATCH"*) if [ -n "$AZ_FAIL_MATCH" ]; then printf '%s\n' "${AZ_FAIL_MESSAGE:-failed}" >&2; exit 1; fi ;;
esac
case "$*" in
  "account show") printf '%s' "${AZ_ACCOUNT_SHOW:-{}}" ;;
  "login") if [ -n "$AZ_LOGIN_FAIL" ]; then exit 1; fi; printf '%s' "{}" ;;
  "account list"*) printf '%s' "$AZ_ACCOUNT_LIST" ;;
  "group list"*) printf '%s' "$AZ_GROUP_LIST" ;;
  "rest"*) case "$*" in *"https://next/page"*) printf '%s' "$AZ_REST_NEXT" ;; *) printf '%s' "$AZ_REST_FIRST" ;; esac ;;
  "policy set-definition show"*) printf '%s' "$AZ_SET_SHOW" ;;
  "policy definition show"*) case "$*" in *"--name z"*) printf '%s' "$AZ_DEF_Z" ;; *"--name a"*) printf '%s' "$AZ_DEF_A" ;; esac ;;
  "policy exemption create"*) printf '%s' "$AZ_CREATE" ;;
esac
`
	path := filepath.Join(dir, "az")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AZ_TEST_LOG", log)
	return log
}

func assertLogContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("az invocation log does not contain %q:\n%s", want, data)
	}
}
