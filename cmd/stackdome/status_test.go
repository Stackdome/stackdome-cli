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
	"testing"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"gopkg.in/yaml.v3"
)

func TestStatusWatchRejectsYAMLStreamOutput(t *testing.T) {
	const stackID = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + stackID + `","name":"app","spec":{}}`))
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1", CurrentStack: stackID}, output.FormatYAML, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	commandContext, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	cmd := newStatusCmd()
	cmd.SetContext(commandContext)
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--watch"})

	err := cmd.Execute()
	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("status --watch YAML error = %T (%v), want validation error", err, err)
	}
	if stdout.Len() != 0 || requests != 0 {
		t.Errorf("stdout = %q, requests = %d; want rejection before watching", stdout.String(), requests)
	}
}

// Status must expose the persisted stack separately from the dynamic live
// status, since the stack document alone cannot tell an agent what is serving.
func TestStatusJSONIncludesStackAndLiveStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"app","spec":{},"converged_release":{"id":"rel-7"}}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-7":
			_, _ = w.Write([]byte(`{"id":"rel-7","live_status":{"health":"ok"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newStatusCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--stack", "app"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status: %v", err)
	}

	var got struct {
		Stack      json.RawMessage `json:"stack"`
		LiveStatus struct {
			Health string `json:"health"`
		} `json:"live_status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("status output is not JSON: %v\nstdout: %s", err, stdout.String())
	}
	if len(got.Stack) == 0 {
		t.Errorf("result omitted stack: %s", stdout.String())
	}
	if got.LiveStatus.Health != "ok" {
		t.Errorf("live_status.health = %q, want ok", got.LiveStatus.Health)
	}
}

func TestStatusYAMLIncludesStackAndLiveStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"app","spec":{},"converged_release":{"id":"rel-7"}}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-7":
			_, _ = w.Write([]byte(`{"id":"rel-7","live_status":{"health":"ok"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}, output.FormatYAML, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newStatusCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--stack", "app"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status: %v", err)
	}

	var got struct {
		Stack map[string]any `yaml:"stack"`
		Live  struct {
			Health string `yaml:"health"`
		} `yaml:"live_status"`
	}
	if err := yaml.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("status output is not YAML: %v\nstdout: %s", err, stdout.String())
	}
	if got.Stack["name"] != "app" || got.Live.Health != "ok" {
		t.Errorf("YAML result = %#v, want stack app and live health ok", got)
	}
}

// A watched status is a stream, so JSON mode must use one compact result per
// line and surface an interruption as cancellation instead of a false success.
func TestWatchStatusJSONWritesNDJSONAndReturnsCancellation(t *testing.T) {
	commandContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	releaseReads := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"app","spec":{},"converged_release":{"id":"rel-7"}}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/releases/rel-7":
			releaseReads++
			health := "ok"
			if releaseReads == 2 {
				health = "progressing"
				time.AfterFunc(10*time.Millisecond, cancel)
			}
			_, _ = w.Write([]byte(`{"id":"rel-7","live_status":{"health":"` + health + `"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newStatusCmd()
	cmd.SetContext(commandContext)
	err := watchStatus(ctx, cmd, "stack-1", false)
	if err != clierrors.ErrUserCanceled {
		t.Fatalf("watchStatus error = %v, want cancellation", err)
	}

	stream := stdout.String()
	if len(stream) == 0 || stream[len(stream)-1] != '\n' {
		t.Fatalf("watch output does not end on a line: %q", stream)
	}
	if bytes.Contains([]byte(stream), []byte("\n  ")) {
		t.Fatalf("watch output is indented rather than compact: %q", stream)
	}
	var lines []struct {
		Stack      json.RawMessage `json:"stack"`
		LiveStatus struct {
			Health string `json:"health"`
		} `json:"live_status"`
	}
	scanner := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	for scanner.Scan() {
		var line struct {
			Stack      json.RawMessage `json:"stack"`
			LiveStatus struct {
				Health string `json:"health"`
			} `json:"live_status"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("watch line is not independent JSON: %v\nline: %s", err, scanner.Text())
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan watch output: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("decoded %d lines, want 2: %s", len(lines), stream)
	}
	if len(lines[0].Stack) == 0 || lines[0].LiveStatus.Health != "ok" || len(lines[1].Stack) == 0 || lines[1].LiveStatus.Health != "progressing" {
		t.Errorf("watch lines = %#v, want ok then progressing live status", lines)
	}
}
