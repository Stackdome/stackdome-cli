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

func TestTokenLoginSchemelessURLUsesHTTPSAndPersistsNormalizedURL(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","email":"agent@example.com","organisation_id":"org-1","role":"OrgAdmin"}`))
		case "/api/v1/users/current/projects":
			_, _ = w.Write([]byte(`{"items":[{"id":"project-1","name":"default","default_project":true}],"total":1}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	serverWithoutScheme := strings.TrimPrefix(ts.URL, "https://")
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{
		"login", "--url", serverWithoutScheme, "--token", "sdm_test", "--insecure", "-o", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	var got authenticationResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not an authentication result: %v\n%s", err, stdout.String())
	}
	if got.ServerURL != ts.URL {
		t.Fatalf("server URL = %q, want normalized HTTPS URL %q", got.ServerURL, ts.URL)
	}
}

func TestTokenLoginRejectsHTTPWithoutInsecureBeforeSendingToken(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"reason":"token rejected"}`))
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	const token = "sdm_must_not_be_sent"
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"login", "--url", ts.URL, "--token", token, "-o", "json"}, &stdout, &stderr)
	if code != 4 {
		t.Fatalf("exit code = %d, want validation exit 4; stderr: %s", code, stderr.String())
	}
	if requests != 0 {
		t.Fatalf("HTTP requests = %d, want rejection before sending credentials", requests)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--insecure") {
		t.Fatalf("stderr = %q, want --insecure remediation", stderr.String())
	}
	if strings.Contains(stderr.String(), token) {
		t.Fatalf("stderr leaked token: %s", stderr.String())
	}
}

func TestTokenLoginDoesNotPrintTokenEchoedByServer(t *testing.T) {
	const token = "sdm_secret_login_token"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"reason":"rejected ` + token + `"}`))
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{
		"login", "--url", ts.URL, "--token", token, "--insecure", "-o", "json",
	}, &stdout, &stderr)
	if code != 4 {
		t.Fatalf("login exit code = %d, want server validation exit 4", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if strings.Contains(stderr.String(), token) {
		t.Fatalf("stderr leaked API token echoed by server: %s", stderr.String())
	}
}

func TestTokenLoginRedactsTokenEchoedByProjectDiscovery(t *testing.T) {
	const token = "sdm_secret_project_lookup_token"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/users/current":
			_, _ = w.Write([]byte(`{"id":"user-1","email":"agent@example.com","organisation_id":"org-1"}`))
		case "/api/v1/users/current/projects":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"reason":"project lookup rejected ` + token + `"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{
		"login", "--url", ts.URL, "--token", token, "--insecure", "-o", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("login exit code = %d, want persisted login success; stderr: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), token) || strings.Contains(stderr.String(), token) {
		t.Fatalf("project discovery leaked API token: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "[REDACTED]") {
		t.Fatalf("stderr = %q, want captured redacted project warning", stderr.String())
	}
}

func TestNormalizeLoginServerURLRejectsOutputSensitiveComponents(t *testing.T) {
	for _, raw := range []string{
		"https://user:secret@stackdome.example",
		"https://stackdome.example?token=secret",
		"https://stackdome.example#secret",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := normalizeLoginServerURL(raw, false); err == nil {
				t.Fatalf("normalizeLoginServerURL(%q) succeeded, want validation error", raw)
			}
		})
	}
}

func TestNormalizeLoginServerURLRemovesTrailingSlash(t *testing.T) {
	got, err := normalizeLoginServerURL("https://stackdome.example/", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://stackdome.example" {
		t.Fatalf("normalized URL = %q, want https://stackdome.example", got)
	}
}
