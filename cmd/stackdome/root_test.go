package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/config"
)

// Removing either command from the root makes documented agent recovery and
// Stackfile discovery unreachable even if their implementation still builds.
func TestRootRegistersAgentLaunchCommands(t *testing.T) {
	root := newRootCmd()
	for _, path := range [][]string{{"doctor"}, {"get", "stackfile-schema"}, {"api"}} {
		command, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		if command == root || !command.Runnable() {
			t.Errorf("agent command %v is not registered", path)
		}
	}
}

// If root error handling regresses to prose in JSON mode, automation can no
// longer decode failures even though stdout correctly contains no result.
func TestRunWithWritersEmitsJSONErrorOnlyOnStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := runWithWriters([]string{"--output", "json", "--not-a-flag"}, &stdout, &stderr)

	if code != 4 {
		t.Fatalf("exit code = %d, want 4", code)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
	var got struct {
		Error    string `json:"error"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("stderr is not a JSON error document: %v\nstderr: %s", err, stderr.String())
	}
	if got.Error == "" {
		t.Error("error document has an empty error")
	}
	if got.ExitCode != 4 {
		t.Errorf("error document exit_code = %d, want 4", got.ExitCode)
	}
}

// Cobra stops parsing at an unknown flag, so JSON error selection must inspect
// the original argv rather than depend on whether the output flag was reached.
func TestRunWithWritersFindsJSONOutputAfterInvalidFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "short separate", args: []string{"--not-a-flag", "-o", "json"}},
		{name: "long separate", args: []string{"--not-a-flag", "--output", "json"}},
		{name: "short equals", args: []string{"--not-a-flag", "-o=json"}},
		{name: "long equals", args: []string{"--not-a-flag", "--output=json"}},
		{name: "short attached", args: []string{"-ojson", "--not-a-flag"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runWithWriters(tt.args, &stdout, &stderr)

			if code == 0 {
				t.Fatal("exit code = 0, want failure")
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			var got struct {
				Error    string `json:"error"`
				ExitCode int    `json:"exit_code"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
				t.Fatalf("stderr is not JSON: %v\nstderr: %s", err, stderr.String())
			}
			if got.Error == "" || got.ExitCode != code {
				t.Errorf("error document = %#v, want non-empty error and exit_code %d", got, code)
			}
		})
	}
}

func TestRunWithWritersShowsLeafHelpWhenRequiredArgsAreMissing(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "table", args: []string{"create", "secret"}},
		{name: "yaml", args: []string{"create", "secret", "--output", "yaml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := runWithWriters(tt.args, &stdout, &stderr)

			if code != 4 {
				t.Fatalf("exit code = %d, want 4", code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			const usage = "Usage:\n  stackdome create secret <name> [flags]"
			if !strings.Contains(stderr.String(), usage) {
				t.Errorf("stderr does not contain command help usage %q:\n%s", usage, stderr.String())
			}
			if !strings.Contains(stderr.String(), "Error: accepts 1 arg(s), received 0") {
				t.Errorf("stderr does not contain validation error:\n%s", stderr.String())
			}
		})
	}
}

func TestRunWithWritersKeepsMissingArgsErrorMachineReadableInJSONMode(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := runWithWriters([]string{"create", "secret", "--output", "json"}, &stdout, &stderr)

	if code != 4 {
		t.Fatalf("exit code = %d, want 4", code)
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
	if got.Error != "accepts 1 arg(s), received 0" || got.ExitCode != 4 {
		t.Errorf("error document = %#v, want missing-argument validation error", got)
	}
}

func TestRunWithWritersLeavesZeroArgumentCommandsUnchanged(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	var stdout, stderr bytes.Buffer

	code := runWithWriters([]string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "stackdome ") {
		t.Errorf("stdout = %q, want version output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

// Cancellation is observable at the process boundary: a JSON follow command
// must retain its one-error-document stderr contract and exit 130 even when
// the release event channel closes quietly as the parent context is cancelled.
func TestRunWithContextReleaseEventsFollowCancellationUsesJSONErrorContract(t *testing.T) {
	const stackID = "11111111-1111-1111-1111-111111111111"
	streamStarted := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+stackID+"/releases":
			_, _ = w.Write([]byte(`{"items":[{"id":"release-follow"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+stackID+"/releases/release-follow/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			close(streamStarted)
			<-r.Context().Done()
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
		CurrentStack:   stackID,
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderr bytes.Buffer
	codeCh := make(chan int, 1)
	go func() {
		codeCh <- runWithContext(parent, []string{"get", "release-events", "release-follow", "--follow", "--output", "json"}, &stdout, &stderr)
	}()

	select {
	case <-streamStarted:
		time.Sleep(20 * time.Millisecond)
		cancel()
	case <-time.After(time.Second):
		t.Fatal("release events stream did not start")
	}

	select {
	case code := <-codeCh:
		if code != 130 {
			t.Fatalf("exit code = %d, want 130", code)
		}
	case <-time.After(time.Second):
		t.Fatal("root command did not return after cancellation")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	var got struct {
		Error    string `json:"error"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("stderr is not a JSON error document: %v\nstderr: %s", err, stderr.String())
	}
	if got.Error != "Aborted." || got.ExitCode != 130 {
		t.Errorf("error document = %#v, want user cancellation", got)
	}
}
