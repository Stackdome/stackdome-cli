package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stackdome/cli/internal/cmdutil"
	"github.com/stackdome/cli/internal/config"
	"github.com/stackdome/cli/internal/output"
)

// Applying a stack only stores the document — the server creates no release for
// it. Deploy must follow the apply with an explicit createRelease, or nothing
// ever rolls out.
func TestDeployAppliesThenCreatesRelease(t *testing.T) {
	var calls []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/apply":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"basic-stack","spec":{}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"rel-7","stack_id":"stack-1","sequence":3,"state":"Pending"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))

	cfg := &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
	}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatTable, slog.LevelError)

	cmd := newDeployCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--file", filepath.Join("..", "..", "internal", "stackfile", "testdata", "basic_image.yaml")})
	cmd.SetOut(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("deploy: %v", err)
	}

	want := []string{
		"PUT /api/v1/organizations/org-1/projects/proj-1/stacks/apply",
		"POST /api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases",
	}
	if len(calls) != len(want) {
		t.Fatalf("expected %v, got %v", want, calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call %d: got %q, want %q", i, calls[i], want[i])
		}
	}

	if cfg.CurrentStack != "stack-1" {
		t.Errorf("current stack = %q, want %q", cfg.CurrentStack, "stack-1")
	}
}
