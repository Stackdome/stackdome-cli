package main

import (
	"bufio"
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
)

func TestLogsRejectsYAMLStreamOutput(t *testing.T) {
	const stackID = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: end\ndata: {}\n\n"))
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1", CurrentStack: stackID}, output.FormatYAML, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newLogsCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)

	err := cmd.Execute()
	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("logs YAML error = %T (%v), want validation error", err, err)
	}
	if stdout.Len() != 0 || requests != 0 {
		t.Errorf("stdout = %q, requests = %d; want rejection before streaming", stdout.String(), requests)
	}
}

// JSON log streams must preserve a JSON payload as data, rather than burying
// it in a quoted string that agents would have to decode a second time.
func TestLogsJSONWritesDecodedNDJSONEvent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/logs":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: log\ndata: {\"message\":\"hello\"}\n\nevent: log\ndata: plain line\n\nevent: end\ndata: {}\n\n"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newLogsCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--stack", "app"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logs: %v", err)
	}

	if bytes.Contains(stdout.Bytes(), []byte("\n  ")) {
		t.Fatalf("log stream is indented instead of compact: %q", stdout.String())
	}
	var lines []struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
	}
	scanner := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	for scanner.Scan() {
		var line struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("log line is not independent JSON: %v\nline: %s", err, scanner.Text())
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan log output: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("decoded %d lines, want 2: %s", len(lines), stdout.String())
	}
	var first struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(lines[0].Data, &first); err != nil {
		t.Fatalf("first data is not decoded JSON: %v", err)
	}
	var second string
	if err := json.Unmarshal(lines[1].Data, &second); err != nil {
		t.Fatalf("second data is not a JSON string: %v", err)
	}
	if lines[0].Event != "log" || first.Message != "hello" || lines[1].Event != "log" || second != "plain line" {
		t.Errorf("lines = %#v, want decoded JSON then raw string data", lines)
	}
}

// Human log output remains the server's raw line so existing terminal and
// shell workflows are unchanged by the JSON stream support.
func TestLogsTableWritesRawData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/logs":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: log\ndata: [web] hello\n\nevent: end\ndata: {}\n\n"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}, output.FormatTable, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newLogsCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--stack", "app"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logs: %v", err)
	}
	if got := stdout.String(); got != "[web] hello\n" {
		t.Errorf("raw logs = %q, want raw server line", got)
	}
}

func TestLogsResourceFlagSelectsRuntimeResourceNamedBuild(t *testing.T) {
	const stackID = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: end\ndata: {}\n\n"))
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{
		ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1",
		ProjectName: "proj-1", CurrentStack: stackID,
	}, output.FormatJSON, slog.LevelError)
	cmd := newLogsCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--resource", "build"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("logs --resource build: %v", err)
	}
	want := "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/resources/build/logs"
	if requestedPath != want {
		t.Errorf("request path = %q, want %q", requestedPath, want)
	}
}

// A server-side SSE error must reach the root error boundary without first
// leaking a prose line that would make JSON stderr undecodable.
func TestLogsJSONServerErrorIsSingleRootDocument(t *testing.T) {
	if os.Getenv("STACKDOME_TEST_LOG_ERROR_HELPER") == "1" {
		os.Exit(runWithWriters([]string{"logs", "--stack", "app", "-o", "json"}, os.Stdout, os.Stderr))
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/logs":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: error\ndata: runtime stream failed\n\n"))
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

	child := exec.Command(os.Args[0], "-test.run=^TestLogsJSONServerErrorIsSingleRootDocument$")
	child.Env = append(os.Environ(), "STACKDOME_TEST_LOG_ERROR_HELPER=1", "STACKDOME_CONFIG="+configPath)
	var stdout, stderr bytes.Buffer
	child.Stdout = &stdout
	child.Stderr = &stderr
	if err := child.Run(); err == nil {
		t.Fatal("logs process succeeded, want server stream failure")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	var got struct {
		Error    string `json:"error"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("stderr is not one JSON document: %v\nstderr: %s", err, stderr.String())
	}
	if got.Error != "runtime stream failed" || got.ExitCode == 0 {
		t.Errorf("error document = %#v, want runtime stream failure", got)
	}
}

// Cancelling an active stream is an interruption, not a stream failure. The
// command must preserve the cancellation sentinel for the root exit boundary.
func TestLogsCancellationAfterStreamStartsIsUserCancellation(t *testing.T) {
	const stackID = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
	started := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1", CurrentStack: stackID}, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := newLogsCmd()
	cmd.SetContext(parent)
	cmdutil.SetContext(cmd, ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.Execute() }()
	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("log stream did not start")
	}
	select {
	case err := <-errCh:
		if err != clierrors.ErrUserCanceled {
			t.Fatalf("cancellation error = %v, want ErrUserCanceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("logs command did not return after cancellation")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

// Exercise the real root boundary without modifying root.go: after a stream
// has started, an interrupt must produce the conventional 130 JSON error.
func TestLogsRootCancellationWritesJSONErrorAndNoStdout(t *testing.T) {
	const stackID = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/organizations/org-1/projects/proj-1/stacks/"+stackID+"/logs" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		cancel()
		<-r.Context().Done()
	}))
	defer ts.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	if err := (&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1", CurrentStack: stackID}).Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if code := runWithContext(parent, []string{"logs", "-o", "json"}, &stdout, &stderr); code != clierrors.ExitUserCanceled {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, clierrors.ExitUserCanceled, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	var result struct {
		Error    string `json:"error"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
		t.Fatalf("stderr is not one JSON document: %v\nstderr: %s", err, stderr.String())
	}
	if result.Error != "Aborted." || result.ExitCode != clierrors.ExitUserCanceled {
		t.Errorf("error document = %#v, want cancellation", result)
	}
}
