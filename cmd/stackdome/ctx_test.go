package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type ctxTestStack struct {
	Name string `json:"name" yaml:"name"`
	ID   string `json:"id" yaml:"id"`
}

type ctxTestResult struct {
	User           string        `json:"user" yaml:"user"`
	Email          string        `json:"email" yaml:"email"`
	ServerURL      string        `json:"server_url" yaml:"server_url"`
	OrganizationID string        `json:"organization_id" yaml:"organization_id"`
	Project        string        `json:"project" yaml:"project"`
	CurrentStack   *ctxTestStack `json:"current_stack" yaml:"current_stack"`
	AuthMethod     string        `json:"auth_method" yaml:"auth_method"`
	TokenSource    string        `json:"token_source" yaml:"token_source"`
}

func TestCtxStructuredOutputResolvesSelectedStack(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			stdout, stderr, code := executeCtx(t, format, "stack-2")
			if code != 0 || stderr != "" {
				t.Fatalf("ctx exit=%d stderr=%q", code, stderr)
			}
			var got ctxTestResult
			decodeCtxOutput(t, format, stdout, &got)
			if got.User != "Ashish" || got.Email != "ashish@example.com" || got.ServerURL == "" || got.OrganizationID != "org-1" || got.Project != "default" {
				t.Fatalf("context = %#v", got)
			}
			if got.CurrentStack == nil || got.CurrentStack.Name != "two" || got.CurrentStack.ID != "stack-2" {
				t.Fatalf("current stack = %#v", got.CurrentStack)
			}
			if got.AuthMethod != "api token" || got.TokenSource != "config file" {
				t.Fatalf("auth = %q from %q", got.AuthMethod, got.TokenSource)
			}
		})
	}
}

func TestCtxTableOutputIncludesContextAndResolvedLegacyStackName(t *testing.T) {
	stdout, stderr, code := executeCtx(t, "table", "two")
	if code != 0 || stderr != "" {
		t.Fatalf("ctx exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{
		"User:", "Ashish <ashish@example.com>", "Server:", "Org:", "org-1",
		"Project:", "default", "Stack:", "two (stack-2)", "Auth:", "api token", "config file",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table output omitted %q:\n%s", want, stdout)
		}
	}
}

func TestCtxWithoutSelectedStackSucceedsAndShowsGuidance(t *testing.T) {
	stdout, stderr, code := executeCtx(t, "table", "")
	if code != 0 || stderr != "" {
		t.Fatalf("ctx exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "Stack:    none") || !strings.Contains(stdout, "stackdome use stack <stack>") {
		t.Fatalf("missing no-stack guidance:\n%s", stdout)
	}
}

func TestCtxJSONOmitsCurrentStackWhenNoneSelected(t *testing.T) {
	stdout, stderr, code := executeCtx(t, "json", "")
	if code != 0 || stderr != "" || strings.Contains(stdout, "current_stack") {
		t.Fatalf("ctx exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestCtxReturnsNotFoundForStaleSelection(t *testing.T) {
	stdout, stderr, code := executeCtx(t, "json", "missing-stack")
	if code != 3 || stdout != "" || !strings.Contains(stderr, `Stack \"missing-stack\" not found`) {
		t.Fatalf("ctx exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestCtxNeverPrintsConfiguredToken(t *testing.T) {
	stdout, stderr, code := executeCtx(t, "json", "stack-2")
	if code != 0 {
		t.Fatalf("ctx exit=%d stderr=%q", code, stderr)
	}
	if strings.Contains(stdout, "sdm_ctx_test") || strings.Contains(stderr, "sdm_ctx_test") {
		t.Fatalf("ctx leaked configured token: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCtxEnvironmentOverrideDoesNotExposeFileStack(t *testing.T) {
	stackRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","name":"Ashish","email":"ashish@example.com","organisation_id":"env-org"}`))
		case "/api/v1/organizations/env-org/projects/env-project/stacks":
			stackRequests++
			_, _ = w.Write([]byte(`{"items":[],"total":0}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	body := fmt.Sprintf(`{"server_url":%q,"access_token":"file-token","organization_id":"file-org","project_name":"file-project","current_stack":"file-stack"}`, server.URL)
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STACKDOME_CONFIG", configPath)
	t.Setenv("STACKDOME_URL", server.URL)
	t.Setenv("STACKDOME_TOKEN", "sdm_env_test")
	t.Setenv("STACKDOME_ORG", "env-org")
	t.Setenv("STACKDOME_PROJECT", "env-project")

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"ctx", "-o", "json"}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("ctx exit=%d stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "current_stack") || strings.Contains(stdout.String(), "file-stack") {
		t.Fatalf("ctx leaked file-backed stack: %s", stdout.String())
	}
	if stackRequests != 0 {
		t.Fatalf("stack-list requests = %d, want zero without an active selection", stackRequests)
	}
}

func executeCtx(t *testing.T, format, current string) (stdout, stderr string, code int) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","name":"Ashish","email":"ashish@example.com","organisation_id":"org-1"}`))
		case "/api/v1/organizations/org-1/projects/default/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"one","spec":{}},{"id":"stack-2","name":"two","spec":{}}],"total":2}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	body := fmt.Sprintf(`{"server_url":%q,"access_token":"sdm_ctx_test","organization_id":"org-1","project_name":"default","current_stack":%q}`, server.URL, current)
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STACKDOME_CONFIG", configPath)

	var stdoutBuffer, stderrBuffer bytes.Buffer
	code = runWithWriters([]string{"ctx", "-o", format}, &stdoutBuffer, &stderrBuffer)
	return stdoutBuffer.String(), stderrBuffer.String(), code
}

func decodeCtxOutput(t *testing.T, format, raw string, target any) {
	t.Helper()
	var err error
	if format == "yaml" {
		err = yaml.Unmarshal([]byte(raw), target)
	} else {
		err = json.Unmarshal([]byte(raw), target)
	}
	if err != nil {
		t.Fatalf("decode ctx %s: %v\n%s", format, err, raw)
	}
}
