package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestTokenLoginJSONPrintsNonSecretAuthenticationResult(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","email":"agent@example.com","username":"Agent","organisation_id":"org-1","role":"OrgAdmin"}`))
		case "/api/v1/users/current/projects":
			_, _ = w.Write([]byte(`{"items":[{"id":"project-1","name":"default","default_project":true}],"total":1}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	const token = "sdm_must_not_appear_in_output"
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"login", "--url", ts.URL, "--token", token, "--insecure", "-o", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty structured success", stderr.String())
	}
	if strings.Contains(stdout.String(), token) {
		t.Fatalf("stdout leaked API token: %s", stdout.String())
	}

	var got authenticationResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a JSON authentication result: %v\nstdout: %s", err, stdout.String())
	}
	if !got.Authenticated || got.User != "Agent" || got.OrganizationID != "org-1" || got.Project != "default" || got.ServerURL != ts.URL || got.AuthMethod != "api_token" {
		t.Errorf("authentication result = %#v", got)
	}
}

func TestLogoutJSONPrintsResult(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"logout", "-o", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty structured success", stderr.String())
	}
	var got struct {
		LoggedOut bool `json:"logged_out"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a JSON logout result: %v\nstdout: %s", err, stdout.String())
	}
	if !got.LoggedOut {
		t.Errorf("logout result = %#v, want logged_out=true", got)
	}
}
