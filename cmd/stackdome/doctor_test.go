package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	"github.com/Stackdome/stackdome-cli/internal/output"
)

func TestDoctorJSONReportsEveryHealthyCheck(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","email":"agent@example.com","organisation_id":"org-1"}`))
		case "/api/v1/users/current/projects":
			_, _ = w.Write([]byte(`{"items":[{"name":"proj-1","default_project":true}],"total":1}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/11111111-1111-1111-1111-111111111111":
			_, _ = w.Write([]byte(`{"id":"11111111-1111-1111-1111-111111111111","name":"demo","spec":{}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	result, err := executeDoctor(t, &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
		CurrentStack:   "11111111-1111-1111-1111-111111111111",
	})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if result.CLI.Version == "" || result.CLI.Status != doctorStatusOK {
		t.Errorf("cli = %#v, want version and ok status", result.CLI)
	}
	if !result.Server.Reachable || result.Server.URL != ts.URL || result.Server.Status != doctorStatusOK {
		t.Errorf("server = %#v", result.Server)
	}
	if !result.Auth.Authenticated || result.Auth.Status != doctorStatusOK {
		t.Errorf("auth = %#v", result.Auth)
	}
	if result.Organization.ID != "org-1" || result.Organization.Status != doctorStatusOK {
		t.Errorf("organization = %#v", result.Organization)
	}
	if result.Project.Name != "proj-1" || !result.Project.Implicit || result.Project.Status != doctorStatusOK {
		t.Errorf("project = %#v", result.Project)
	}
	if result.Stack.ID != "11111111-1111-1111-1111-111111111111" || result.Stack.Status != doctorStatusOK {
		t.Errorf("stack = %#v", result.Stack)
	}
	if result.Compatibility.Status != doctorStatusUnknown {
		t.Errorf("compatibility = %#v, want unknown", result.Compatibility)
	}
}

func TestDoctorFailsConfiguredOrganizationThatDoesNotMatchAuthenticatedUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","email":"agent@example.com","organisation_id":"org-actual"}`))
		case "/api/v1/users/current/projects":
			_, _ = w.Write([]byte(`{"items":[{"name":"default","default_project":true}],"total":1}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	result, err := executeDoctor(t, &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-configured",
		ProjectName:    "default",
	})
	if err == nil {
		t.Fatal("doctor succeeded with an organization that does not match the authenticated user")
	}
	if result.Organization.Status != doctorStatusFail {
		t.Fatalf("organization = %#v, want failed", result.Organization)
	}
	if result.Organization.Error == "" {
		t.Error("organization mismatch omitted its failure detail")
	}
}

func TestDoctorFailsConfiguredProjectThatIsNotSignupDefault(t *testing.T) {
	var projectLookups int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","email":"agent@example.com","organisation_id":"org-1"}`))
		case "/api/v1/users/current/projects":
			projectLookups++
			_, _ = w.Write([]byte(`{"items":[{"name":"signup-default","default_project":true}],"total":1}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	result, err := executeDoctor(t, &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "stale-project",
	})
	if err == nil {
		t.Fatal("doctor succeeded with a configured project that is not the signup default")
	}
	if projectLookups != 1 {
		t.Fatalf("default project lookups = %d, want 1", projectLookups)
	}
	if result.Project.Status != doctorStatusFail || result.Project.Error == "" {
		t.Errorf("project = %#v, want failed with detail", result.Project)
	}
}

func TestDoctorAcceptsConfiguredScopeWhenProjectDiscoveryIsForbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","email":"agent@example.com","organisation_id":"org-1"}`))
		case "/api/v1/users/current/projects":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"reason":"project read scope denied"}`))
		case "/api/v1/organizations/org-1/projects/configured-default/stacks":
			_, _ = w.Write([]byte(`{"items":[],"total":0}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	result, err := executeDoctor(t, &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_scoped",
		OrganizationID: "org-1",
		ProjectName:    "configured-default",
	})
	if err != nil {
		t.Fatalf("doctor rejected usable configured scope: %v", err)
	}
	if result.Auth.Status != doctorStatusOK || result.Project.Status != doctorStatusOK {
		t.Errorf("auth/project = %#v / %#v, want usable configured scope", result.Auth, result.Project)
	}
}

func TestDoctorAcceptsConfiguredScopeWhenCurrentUserDiscoveryIsForbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/v1/users/current":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"reason":"user discovery scope denied"}`))
		case "/api/v1/organizations/org-1/projects/configured-default/stacks/11111111-1111-1111-1111-111111111111":
			_, _ = w.Write([]byte(`{"id":"11111111-1111-1111-1111-111111111111","name":"demo","spec":{}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	result, err := executeDoctor(t, &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_scoped",
		OrganizationID: "org-1",
		ProjectName:    "configured-default",
		CurrentStack:   "11111111-1111-1111-1111-111111111111",
	})
	if err != nil {
		t.Fatalf("doctor rejected usable stack-scoped token: %v", err)
	}
	if result.Auth.Status != doctorStatusOK || !result.Auth.Authenticated {
		t.Errorf("auth = %#v, want ok scoped validation", result.Auth)
	}
	if result.Organization.Status != doctorStatusOK || result.Project.Status != doctorStatusOK {
		t.Errorf("scope = org %#v project %#v, want ok", result.Organization, result.Project)
	}
	if result.Stack.Status != doctorStatusOK || result.Stack.Name != "demo" {
		t.Errorf("stack = %#v, want validated current stack", result.Stack)
	}
}

func TestDoctorReportsFailureWhenDiscoveryAndConfiguredScopeAreForbidden(t *testing.T) {
	var scopeChecks int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","email":"agent@example.com","organisation_id":"org-1"}`))
		case "/api/v1/users/current/projects":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"reason":"project read scope denied"}`))
		case "/api/v1/organizations/org-1/projects/configured-default/stacks":
			scopeChecks++
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"reason":"configured scope denied"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	result, err := executeDoctor(t, &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_scoped",
		OrganizationID: "org-1",
		ProjectName:    "configured-default",
	})
	if err == nil {
		t.Fatal("doctor succeeded when discovery and configured scope were forbidden")
	}
	if scopeChecks != 1 {
		t.Errorf("configured scope checks = %d, want 1", scopeChecks)
	}
	if result.Project.Status != doctorStatusFail || result.Project.Error == "" {
		t.Errorf("project = %#v, want visible scope failure", result.Project)
	}
	if !strings.Contains(result.Project.Error, "configured scope denied") {
		t.Errorf("project error = %q, want scoped API detail", result.Project.Error)
	}
}

func TestDoctorTableRendersFailureDetail(t *testing.T) {
	ctx := &cmdutil.CommandContext{Formatter: output.NewFormatter(output.FormatTable)}
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	renderDoctorTable(ctx, doctorResult{
		CLI:           doctorCLI{doctorCheck: doctorCheck{Status: doctorStatusOK}, Version: "dev"},
		Server:        doctorServer{doctorCheck: doctorCheck{Status: doctorStatusFail, Error: "dial refused"}},
		Auth:          doctorAuth{doctorCheck: doctorCheck{Status: doctorStatusFail, Error: "token expired"}},
		Organization:  doctorOrganization{doctorCheck: doctorCheck{Status: doctorStatusFail, Error: "organization mismatch"}},
		Project:       doctorProject{doctorCheck: doctorCheck{Status: doctorStatusFail, Error: "project scope denied"}},
		Stack:         doctorStack{doctorCheck: doctorCheck{Status: doctorStatusSkipped}},
		Compatibility: doctorCompatibility{doctorCheck: doctorCheck{Status: doctorStatusUnknown}, Detail: "metadata unavailable"},
	})

	for _, detail := range []string{"dial refused", "token expired", "organization mismatch", "project scope denied"} {
		if !bytes.Contains(stdout.Bytes(), []byte(detail)) {
			t.Errorf("table omitted %q:\n%s", detail, stdout.String())
		}
	}
}

func TestDoctorReturnsErrorAfterReportingFailingChecks(t *testing.T) {
	result, err := executeDoctor(t, &config.Config{ServerURL: "http://127.0.0.1:1"})
	if err == nil {
		t.Fatal("doctor succeeded despite unreachable server and missing credentials")
	}
	if result.Server.Status != doctorStatusFail {
		t.Errorf("server = %#v, want failed reachability", result.Server)
	}
	if result.Auth.Status != doctorStatusFail {
		t.Errorf("auth = %#v, want failed authentication", result.Auth)
	}
	if result.Organization.Status != doctorStatusFail || result.Project.Status != doctorStatusFail {
		t.Errorf("scope checks = org %#v project %#v, want failures", result.Organization, result.Project)
	}
	if result.Compatibility.Status != doctorStatusUnknown {
		t.Errorf("compatibility = %#v, want unknown", result.Compatibility)
	}
}

func executeDoctor(t *testing.T, cfg *config.Config) (doctorResult, error) {
	t.Helper()
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newDoctorCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	err := cmd.Execute()

	var result doctorResult
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("doctor JSON: %v\nstdout: %s", decodeErr, stdout.String())
	}
	return result, err
}
