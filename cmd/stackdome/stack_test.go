package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	"github.com/Stackdome/stackdome-cli/internal/output"
)

func TestStackUseSelectsExistingStackAndPersistsFullID(t *testing.T) {
	const stackID = "f8ac5eee-e489-44be-955e-7b90f3cd2a07"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/organizations/org-1/projects/default/stacks" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"` + stackID + `","name":"n8n","spec":{}}],"total":1}`))
	}))
	defer ts.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	cfg := &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "default",
	}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newStackCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"use", "n8n"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stack use: %v", err)
	}
	if cfg.CurrentStack != stackID {
		t.Fatalf("current stack = %q, want full ID %q", cfg.CurrentStack, stackID)
	}
	persisted, err := config.LoadFrom(configPath)
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if persisted.CurrentStack != stackID {
		t.Fatalf("persisted current stack = %q, want %q", persisted.CurrentStack, stackID)
	}

	var got mutationResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a JSON selection result: %v\n%s", err, stdout.String())
	}
	if got.Status != "selected" || got.Resource != "stack" || got.Name != "n8n" || got.ID != stackID {
		t.Errorf("selection result = %#v", got)
	}
}

func TestStackUseResolvesMissingPersistedScope(t *testing.T) {
	const stackID = "f8ac5eee-e489-44be-955e-7b90f3cd2a07"
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","email":"agent@example.com","organisation_id":"org-1"}`))
		case "/api/v1/users/current/projects":
			_, _ = w.Write([]byte(`{"items":[{"name":"default","default_project":true}],"total":1}`))
		case "/api/v1/organizations/org-1/projects/default/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"` + stackID + `","name":"n8n","spec":{}}],"total":1}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	body := `{"server_url":"` + ts.URL + `","access_token":"file-token"}`
	if err := os.WriteFile(configPath, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STACKDOME_CONFIG", configPath)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newStackCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"use", "n8n"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stack use: %v; paths: %v", err, paths)
	}
	if cfg.OrganizationID != "org-1" || cfg.ProjectName != "default" || cfg.CurrentStack != stackID {
		t.Fatalf("resolved context = org %q, project %q, stack %q", cfg.OrganizationID, cfg.ProjectName, cfg.CurrentStack)
	}
	wantPaths := []string{
		"/api/v1/users/current",
		"/api/v1/users/current/projects",
		"/api/v1/organizations/org-1/projects/default/stacks",
	}
	if strings.Join(paths, "\n") != strings.Join(wantPaths, "\n") {
		t.Fatalf("paths = %v, want %v", paths, wantPaths)
	}
}

func TestStackSelectAliasIsDiscoverable(t *testing.T) {
	cmd := newStackCmd()
	found, _, err := cmd.Find([]string{"select"})
	if err != nil {
		t.Fatalf("find stack select alias: %v", err)
	}
	if found.Name() != "use" {
		t.Fatalf("stack select resolves to %q, want use", found.Name())
	}
}

func TestStackUseWithEnvironmentTokenRejectsBeforeScopeDiscovery(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("STACKDOME_URL", ts.URL)
	t.Setenv("STACKDOME_TOKEN", "sdm_ephemeral")

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"use", "stack", "n8n", "-o", "json"}, &stdout, &stderr)
	if code != 4 {
		t.Fatalf("exit code = %d, want validation exit 4; stderr: %s", code, stderr.String())
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want zero before rejecting ephemeral selection", requests)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--stack") {
		t.Fatalf("stderr = %q, want --stack remediation", stderr.String())
	}
}

func TestStackUseRejectsEnvironmentSelectionOverridesWithoutChangingFileContext(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value func(string) string
	}{
		{name: "server", key: "STACKDOME_URL", value: func(serverURL string) string { return serverURL }},
		{name: "organization", key: "STACKDOME_ORG", value: func(string) string { return "env-org" }},
		{name: "project", key: "STACKDOME_PROJECT", value: func(string) string { return "env-project" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"items":[{"id":"env-stack","name":"n8n","spec":{}}],"total":1}`))
			}))
			defer ts.Close()

			configPath := filepath.Join(t.TempDir(), "config.json")
			body := `{"server_url":"` + ts.URL + `","access_token":"file-token","organization_id":"file-org","project_name":"default","current_stack":"file-stack"}`
			if err := os.WriteFile(configPath, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("STACKDOME_CONFIG", configPath)
			t.Setenv(tt.key, tt.value(ts.URL))

			var stdout, stderr bytes.Buffer
			code := runWithWriters([]string{"use", "stack", "n8n", "-o", "json"}, &stdout, &stderr)
			if code != 4 {
				t.Fatalf("exit code = %d, want validation exit 4; stderr: %s", code, stderr.String())
			}
			if requests != 0 {
				t.Fatalf("requests = %d, want zero before rejecting environment override", requests)
			}
			if !strings.Contains(stderr.String(), "--stack") {
				t.Fatalf("stderr = %q, want --stack remediation", stderr.String())
			}

			persisted, err := config.LoadFrom(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.CurrentStack != "file-stack" || persisted.OrganizationID != "file-org" || persisted.ProjectName != "default" {
				t.Fatalf("file context changed: %#v", persisted)
			}
		})
	}
}

func TestStackInfoResolvesIDReferencesBeforeFetchingDetails(t *testing.T) {
	const stackID = "f8ac5eee-e489-44be-955e-7b90f3cd2a07"
	tests := []struct {
		name string
		ref  string
	}{
		{name: "name", ref: "n8n"},
		{name: "full ID", ref: stackID},
		{name: "unique ID prefix", ref: "f8ac5eee"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var paths []string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v1/organizations/org-1/projects/default/stacks":
					_, _ = w.Write([]byte(`{"items":[{"id":"` + stackID + `","name":"n8n","spec":{}}],"total":1}`))
				case "/api/v1/organizations/org-1/projects/default/stacks/" + stackID:
					_, _ = w.Write([]byte(`{"id":"` + stackID + `","name":"n8n","spec":{}}`))
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer ts.Close()

			cfg := &config.Config{
				ServerURL:      ts.URL,
				AccessToken:    "sdm_test",
				OrganizationID: "org-1",
				ProjectName:    "default",
			}
			ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
			var stdout bytes.Buffer
			ctx.Formatter.Writer = &stdout

			cmd := newStackInfoCmd()
			cmd.SetContext(context.Background())
			cmdutil.SetContext(cmd, ctx)
			cmd.SetArgs([]string{tt.ref})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("stack info %q: %v; paths: %v", tt.ref, err, paths)
			}

			var got struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("stdout is not stack JSON: %v\nstdout: %s", err, stdout.String())
			}
			if got.ID != stackID || got.Name != "n8n" {
				t.Errorf("stack info result = %#v, want n8n %s", got, stackID)
			}
			wantPaths := []string{
				"/api/v1/organizations/org-1/projects/default/stacks",
				"/api/v1/organizations/org-1/projects/default/stacks/" + stackID,
			}
			if strings.Join(paths, "\n") != strings.Join(wantPaths, "\n") {
				t.Fatalf("paths = %v, want %v", paths, wantPaths)
			}
		})
	}
}

func TestStackDeleteJSONPrintsStructuredResult(t *testing.T) {
	var deleted bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/default/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}],"total":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/default/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"app","spec":{}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/organizations/org-1/projects/default/stacks/stack-1":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "default",
	}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newStackDeleteCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"stack-1", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("stack delete: %v", err)
	}
	if !deleted {
		t.Fatal("stack delete did not call the API")
	}

	var got struct {
		Status   string `json:"status"`
		Resource string `json:"resource"`
		Name     string `json:"name"`
		ID       string `json:"id"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a JSON result: %v\nstdout: %s", err, stdout.String())
	}
	if got.Status != "deletion_initiated" || got.Resource != "stack" || got.Name != "app" || got.ID != "stack-1" {
		t.Errorf("result = %#v, want deletion_initiated stack app stack-1", got)
	}
}
