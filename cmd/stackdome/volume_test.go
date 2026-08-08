package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	"github.com/Stackdome/stackdome-cli/internal/output"
)

func TestVolumeCreateUsesCurrentStackEndpoint(t *testing.T) {
	const stackID = "11111111-1111-1111-1111-111111111111"
	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/organizations/org-1/projects/proj-1/stacks/"+stackID+"/volumes" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"id":"volume-1","name":"test-cli","spec":{"size":"1Gi","access_mode":"ReadWriteOnce"}}`))
	}))
	defer ts.Close()

	cfg := &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
		CurrentStack:   stackID,
	}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newVolumeCreateCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"test-cli", "--size", "1Gi"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("volume create: %v; requested path: %s", err, requestedPath)
	}

	var got struct {
		Name string `json:"name"`
		Spec struct {
			Size string `json:"size"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not volume JSON: %v\n%s", err, stdout.String())
	}
	if got.Name != "test-cli" || got.Spec.Size != "1Gi" {
		t.Fatalf("volume = %#v, want test-cli/1Gi", got)
	}
}

func TestVolumeCreateStackFlagOverridesCurrentContext(t *testing.T) {
	const targetStackID = "22222222-2222-2222-2222-222222222222"
	var postedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"` + targetStackID + `","name":"target","spec":{}}],"total":1}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+targetStackID+"/volumes":
			postedPath = r.URL.Path
			_, _ = w.Write([]byte(`{"id":"volume-1","name":"test-cli","spec":{"size":"1Gi","access_mode":"ReadWriteOnce"}}`))
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
		ProjectName:    "proj-1",
		CurrentStack:   "11111111-1111-1111-1111-111111111111",
	}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	ctx.Formatter.Writer = &bytes.Buffer{}

	cmd := newVolumeCreateCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"test-cli", "--size", "1Gi", "--stack", "target"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("volume create with --stack: %v", err)
	}
	wantPath := "/api/v1/organizations/org-1/projects/proj-1/stacks/" + targetStackID + "/volumes"
	if postedPath != wantPath {
		t.Fatalf("POST path = %q, want %q", postedPath, wantPath)
	}
}

func TestVolumeCreateHelpExplainsCurrentStackScope(t *testing.T) {
	cmd := newVolumeCreateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("volume create help: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Create a volume in the current stack")) {
		t.Fatalf("help does not explain current-stack scope:\n%s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("--stack")) {
		t.Fatalf("help does not show stack override:\n%s", stdout.String())
	}
}
