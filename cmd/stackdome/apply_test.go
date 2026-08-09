package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/config"
)

func TestApplySavesStackWithoutCreatingRelease(t *testing.T) {
	var calls []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/apply" {
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"basic-stack","spec":{}}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	cfg := &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	stackfilePath, err := filepath.Abs(filepath.Join("..", "..", "internal", "stackfile", "testdata", "basic_image.yaml"))
	if err != nil {
		t.Fatalf("absolute stackfile path: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"apply", "--file", stackfilePath, "--output", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("apply exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if len(calls) != 1 || calls[0] != "PUT /api/v1/organizations/org-1/projects/proj-1/stacks/apply" {
		t.Fatalf("requests = %v, want one stack apply and no release", calls)
	}
	var got struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a stack JSON object: %v\nstdout: %s", err, stdout.String())
	}
	if got.ID != "stack-1" || got.Name != "basic-stack" {
		t.Errorf("apply result = %#v", got)
	}

	reloaded, err := config.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.CurrentStack != "stack-1" {
		t.Errorf("current stack = %q, want stack-1", reloaded.CurrentStack)
	}
}

func TestCreateReleaseUsesSavedStackWithoutApplying(t *testing.T) {
	var calls []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"demo","spec":{}}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"release-1","stack_id":"stack-1","sequence":1,"state":"Pending"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	cfg := &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"create", "release", "--stack", "demo", "--output", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("create release exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	wantCalls := []string{
		"GET /api/v1/organizations/org-1/projects/proj-1/stacks",
		"POST /api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases",
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("requests = %v, want %v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Errorf("request %d = %q, want %q", i, calls[i], wantCalls[i])
		}
	}
	var got struct {
		Release struct {
			ID string `json:"id"`
		} `json:"release"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not release JSON: %v\nstdout: %s", err, stdout.String())
	}
	if got.Release.ID != "release-1" {
		t.Errorf("release ID = %q, want release-1", got.Release.ID)
	}
}
