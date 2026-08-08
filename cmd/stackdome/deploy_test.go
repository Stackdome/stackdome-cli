package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"gopkg.in/yaml.v3"
)

func TestDeployJSONFailureHasSingleRootErrorDocument(t *testing.T) {
	if os.Getenv("STACKDOME_TEST_DEPLOY_JSON_FAILURE_HELPER") == "1" {
		stackfilePath := filepath.Join("..", "..", "internal", "stackfile", "testdata", "basic_image.yaml")
		os.Exit(runWithWriters([]string{"deploy", "--file", stackfilePath, "-o", "json"}, os.Stdout, os.Stderr))
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/apply":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"basic-stack","spec":{}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"release creation failed"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	child := exec.Command(os.Args[0], "-test.run=^TestDeployJSONFailureHasSingleRootErrorDocument$")
	child.Env = append(os.Environ(), "STACKDOME_TEST_DEPLOY_JSON_FAILURE_HELPER=1", "STACKDOME_CONFIG="+configPath)
	var stdout, stderr bytes.Buffer
	child.Stdout = &stdout
	child.Stderr = &stderr
	if err := child.Run(); err == nil {
		t.Fatal("deploy process succeeded, want release creation failure")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	var got struct {
		Error    string `json:"error"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("stderr is not one JSON error document: %v\nstderr: %s", err, stderr.String())
	}
	if got.Error == "" || got.ExitCode == 0 {
		t.Errorf("error document = %#v, want a failure", got)
	}
}

func TestDeployWaitStructuredKeepsHumanProgressOffStderr(t *testing.T) {
	if format := os.Getenv("STACKDOME_TEST_DEPLOY_STRUCTURED_WAIT_HELPER"); format != "" {
		stackfilePath := filepath.Join("..", "..", "internal", "stackfile", "testdata", "basic_image.yaml")
		os.Exit(runWithWriters([]string{"deploy", "--wait", "--file", stackfilePath, "-o", format}, os.Stdout, os.Stderr))
	}

	var releaseReads int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/apply":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"basic-stack","spec":{}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"rel-7","stack_id":"stack-1","sequence":3,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-7/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: message\ndata: {\"sequence\":1,\"message\":\"building\"}\n\nevent: end\ndata: {}\n\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-7":
			releaseReads++
			if releaseReads%2 == 1 {
				_, _ = w.Write([]byte(`{"id":"rel-7","stack_id":"stack-1","state":"Released"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"rel-7","stack_id":"stack-1","state":"Released","live_status":{"health":"ok"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"basic-stack","spec":{},"converged_release":{"id":"rel-7"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			child := exec.Command(os.Args[0], "-test.run=^TestDeployWaitStructuredKeepsHumanProgressOffStderr$")
			child.Env = append(os.Environ(), "STACKDOME_TEST_DEPLOY_STRUCTURED_WAIT_HELPER="+format, "STACKDOME_CONFIG="+configPath)
			var stdout, stderr bytes.Buffer
			child.Stdout = &stdout
			child.Stderr = &stderr
			if err := child.Run(); err != nil {
				t.Fatalf("deploy process: %v\nstderr: %s", err, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty in structured mode", stderr.String())
			}
			var got deployResult
			if format == "json" {
				if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
					t.Fatalf("stdout is not one deploy JSON document: %v\nstdout: %s", err, stdout.String())
				}
			} else if err := yaml.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("stdout is not one deploy YAML document: %v\nstdout: %s", err, stdout.String())
			}
			if got.Stack == nil || got.Release == nil || got.LiveStatus == nil {
				t.Errorf("deploy result omitted fields: %#v", got)
			}
		})
	}
}

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
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

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
	var result struct {
		Stack   json.RawMessage `json:"stack"`
		Release json.RawMessage `json:"release"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("deploy JSON is invalid: %v\nstdout: %s", err, stdout.String())
	}
	if len(result.Stack) == 0 || len(result.Release) == 0 {
		t.Errorf("deploy JSON omitted stack or release: %s", stdout.String())
	}
}

// A stalled event stream must not leave an agent blocked forever. The
// command's timeout should cancel the request and report a timeout rather than
// treating a deadline as a user interrupt.
func TestDeployWaitTimeoutBoundsEventStream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/apply":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"basic-stack","spec":{}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"rel-timeout","stack_id":"stack-1","sequence":4,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-timeout/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			<-r.Context().Done()
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newDeployCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--wait", "--timeout", "20ms", "--file", filepath.Join("..", "..", "internal", "stackfile", "testdata", "basic_image.yaml")})

	started := time.Now()
	err := cmd.Execute()
	if err == nil {
		t.Fatal("deploy --wait returned nil, want timeout error")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("deploy timeout took %s, want under 1s", elapsed)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want no partial result on timeout", got)
	}
	if err == clierrors.ErrUserCanceled {
		t.Fatalf("deadline reported as user cancellation: %v", err)
	}
}

func TestDeployWaitTimeoutBeforeStreamHeadersUsesTimeoutContract(t *testing.T) {
	streamStarted := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/apply":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"basic-stack","spec":{}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"rel-timeout","stack_id":"stack-1","sequence":4,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-timeout/events/stream":
			close(streamStarted)
			<-r.Context().Done()
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newDeployCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--wait", "--timeout", "20ms", "--file", filepath.Join("..", "..", "internal", "stackfile", "testdata", "basic_image.yaml")})

	err := cmd.Execute()
	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("timeout error type = %T (%v), want *CLIError", err, err)
	}
	if cliErr.Code != "TIMEOUT" {
		t.Fatalf("timeout code = %q, want TIMEOUT (error: %v)", cliErr.Code, err)
	}
	if cliErr.Message != "Timed out waiting for the release to finish." {
		t.Errorf("timeout message = %q", cliErr.Message)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no partial result", stdout.String())
	}
	select {
	case <-streamStarted:
	default:
		t.Fatal("test did not reach the unflushed event stream")
	}
}

// A waited JSON deploy is an observation boundary: consumers need both the
// terminal release and the runtime state that was fetched after it completed.
func TestDeployWaitJSONIncludesLiveStatus(t *testing.T) {
	var releaseReads int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/apply":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"basic-stack","spec":{}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"rel-7","stack_id":"stack-1","sequence":3,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-7/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: end\ndata: {}\n\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-7":
			releaseReads++
			if releaseReads == 1 {
				_, _ = w.Write([]byte(`{"id":"rel-7","stack_id":"stack-1","state":"Released"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"rel-7","stack_id":"stack-1","state":"Released","live_status":{"health":"ok"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"basic-stack","spec":{},"converged_release":{"id":"rel-7"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newDeployCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--wait", "--file", filepath.Join("..", "..", "internal", "stackfile", "testdata", "basic_image.yaml")})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("deploy --wait: %v", err)
	}

	var got struct {
		Stack      json.RawMessage `json:"stack"`
		Release    json.RawMessage `json:"release"`
		LiveStatus struct {
			Health string `json:"health"`
		} `json:"live_status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("deploy output is not JSON: %v\nstdout: %s", err, stdout.String())
	}
	if len(got.Stack) == 0 || len(got.Release) == 0 {
		t.Fatalf("result omitted stack or release: %s", stdout.String())
	}
	if got.LiveStatus.Health != "ok" {
		t.Errorf("live_status.health = %q, want ok", got.LiveStatus.Health)
	}
}

// The wait deadline covers the final stack observation as well as the event
// stream. A slow GetStack must not leak a transport error or a partial result.
func TestDeployWaitTimeoutDuringFinalStackObservationUsesTimeoutContract(t *testing.T) {
	stackFetchStarted := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/apply":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"basic-stack","spec":{}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"rel-timeout","stack_id":"stack-1","sequence":4,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-timeout/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: end\ndata: {}\n\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-timeout":
			_, _ = w.Write([]byte(`{"id":"rel-timeout","stack_id":"stack-1","state":"Released"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			close(stackFetchStarted)
			<-r.Context().Done()
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newDeployCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--wait", "--timeout", "20ms", "--file", filepath.Join("..", "..", "internal", "stackfile", "testdata", "basic_image.yaml")})

	err := cmd.Execute()
	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("timeout error type = %T (%v), want *CLIError", err, err)
	}
	if cliErr.Code != "TIMEOUT" {
		t.Errorf("timeout code = %q, want TIMEOUT (error: %v)", cliErr.Code, err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no partial result", stdout.String())
	}
	select {
	case <-stackFetchStarted:
	default:
		t.Fatal("test did not reach final stack fetch")
	}
}
