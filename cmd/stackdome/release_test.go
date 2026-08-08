package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
)

const rollbackTestStackID = "11111111-1111-1111-1111-111111111111"

func TestReleaseRollbackCreatesReleaseFromHistoricalRelease(t *testing.T) {
	var rollbackBody struct {
		FromReleaseID string `json:"from_release_id"`
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/11111111-1111-1111-1111-111111111111/releases":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"items":[{"id":"release-previous"}]}`))
				return
			}
			if r.Method == http.MethodPost {
				if err := json.NewDecoder(r.Body).Decode(&rollbackBody); err != nil {
					t.Fatalf("decode rollback request: %v", err)
				}
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"11111111-1111-1111-1111-111111111111","sequence":7,"state":"Pending"}`))
				return
			}
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
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
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newReleaseCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"rollback", "release-previous"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("release rollback: %v", err)
	}

	if rollbackBody.FromReleaseID != "release-previous" {
		t.Errorf("from_release_id = %q, want %q", rollbackBody.FromReleaseID, "release-previous")
	}
	var result struct {
		Release struct {
			ID string `json:"id"`
		} `json:"release"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("rollback JSON: %v\nstdout: %s", err, stdout.String())
	}
	if result.Release.ID != "release-rollback" {
		t.Errorf("release.id = %q, want %q", result.Release.ID, "release-rollback")
	}

}

func TestReleaseRollbackWaitJSONIncludesTerminalReleaseAndLiveStatus(t *testing.T) {
	var releaseReads int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			_, _ = w.Write([]byte(`{"items":[{"id":"release-previous"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","sequence":7,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: end\ndata: {}\n\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback":
			releaseReads++
			if releaseReads == 1 {
				_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Released"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Released","live_status":{"health":"ok"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID:
			_, _ = w.Write([]byte(`{"id":"` + rollbackTestStackID + `","name":"demo","spec":{},"converged_release":{"id":"release-rollback"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx, stdout := rollbackCommandContext(ts.URL)
	cmd := newReleaseCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"rollback", "release-previous", "--wait"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("release rollback --wait: %v", err)
	}

	var result struct {
		Release struct {
			ID    string `json:"id"`
			State string `json:"state"`
		} `json:"release"`
		LiveStatus struct {
			Health string `json:"health"`
		} `json:"live_status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("rollback JSON: %v\nstdout: %s", err, stdout.String())
	}
	if result.Release.ID != "release-rollback" || result.Release.State != "Released" {
		t.Errorf("release = %#v, want terminal rollback release", result.Release)
	}
	if result.LiveStatus.Health != "ok" {
		t.Errorf("live_status.health = %q, want ok", result.LiveStatus.Health)
	}
}

func TestReleaseRollbackWaitContinuesAfterStreamEndsWhileReleaseIsPending(t *testing.T) {
	var releaseReads int
	var streamReads int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			_, _ = w.Write([]byte(`{"items":[{"id":"release-previous"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","sequence":7,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback/events/stream":
			streamReads++
			wantAfter := "0"
			sequence := "3"
			if streamReads == 2 {
				wantAfter = "3"
				sequence = "4"
			}
			if got := r.URL.Query().Get("after_sequence"); got != wantAfter {
				t.Errorf("event stream %d after_sequence = %q, want %q", streamReads, got, wantAfter)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: message\ndata: {\"sequence\":" + sequence + ",\"message\":\"progress\"}\n\nevent: end\ndata: {}\n\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback":
			releaseReads++
			switch releaseReads {
			case 1:
				_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Pending"}`))
			case 2:
				_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Released"}`))
			default:
				_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Released","live_status":{"health":"ok"}}`))
			}
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID:
			_, _ = w.Write([]byte(`{"id":"` + rollbackTestStackID + `","name":"demo","spec":{},"converged_release":{"id":"release-rollback"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx, stdout := rollbackCommandContext(ts.URL)
	cmd := newReleaseCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"rollback", "release-previous", "--wait"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("release rollback --wait: %v", err)
	}

	if streamReads != 2 {
		t.Errorf("event stream requests = %d, want 2", streamReads)
	}
	if releaseReads != 3 {
		t.Errorf("release reads = %d, want 3", releaseReads)
	}
	var result struct {
		Release struct {
			State string `json:"state"`
		} `json:"release"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("rollback JSON: %v\nstdout: %s", err, stdout.String())
	}
	if result.Release.State != "Released" {
		t.Errorf("release state = %q, want Released", result.Release.State)
	}
}

// A waited rollback has the same observation contract as a waited deploy: the
// newly-created release must be the one serving, be healthy, and expose every
// declared public port. These cases would all have been reported as success
// when rollback printed the post-release fetch without verifying it.
func TestReleaseRollbackWaitVerifiesDeploymentObservation(t *testing.T) {
	tests := []struct {
		name          string
		stackResponse string
		liveResponse  string
		wantErr       bool
	}{
		{
			name:          "converged release is not the new rollback release",
			stackResponse: `{"id":"` + rollbackTestStackID + `","name":"demo","spec":{},"converged_release":{"id":"release-other"}}`,
			liveResponse:  `{"id":"release-other","stack_id":"` + rollbackTestStackID + `","state":"Released","live_status":{"health":"ok"}}`,
			wantErr:       true,
		},
		{
			name:          "runtime health is degraded",
			stackResponse: `{"id":"` + rollbackTestStackID + `","name":"demo","spec":{},"converged_release":{"id":"release-rollback"}}`,
			liveResponse:  `{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Released","live_status":{"health":"degraded"}}`,
			wantErr:       true,
		},
		{
			name:          "declared public port has no URL",
			stackResponse: `{"id":"` + rollbackTestStackID + `","name":"demo","spec":{"stack_resources":[{"name":"web","ports":[{"name":"http","number":80,"exposed_to_public":true}]}]},"converged_release":{"id":"release-rollback"}}`,
			liveResponse:  `{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Released","live_status":{"health":"ok","resources":{"web":{}}}}`,
			wantErr:       true,
		},
		{
			name:          "private port does not require a URL",
			stackResponse: `{"id":"` + rollbackTestStackID + `","name":"demo","spec":{"stack_resources":[{"name":"web","ports":[{"name":"http","number":80,"exposed_to_public":false}]}]},"converged_release":{"id":"release-rollback"}}`,
			liveResponse:  `{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Released","live_status":{"health":"ok","resources":{"web":{}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := rollbackObservationServer(t, tt.stackResponse, tt.liveResponse)
			defer ts.Close()

			ctx, stdout := rollbackCommandContext(ts.URL)
			cmd := newReleaseCmd()
			cmd.SetContext(context.Background())
			cmdutil.SetContext(cmd, ctx)
			cmd.SetArgs([]string{"rollback", "release-previous", "--wait"})
			err := cmd.Execute()

			if tt.wantErr {
				if err == nil {
					t.Fatal("release rollback --wait returned nil, want verification failure")
				}
				if clierrors.ExitCodeFrom(err) != clierrors.ExitGeneral {
					t.Errorf("exit code = %d (%v), want general verification failure", clierrors.ExitCodeFrom(err), err)
				}
				if stdout.Len() != 0 {
					t.Fatalf("stdout = %q, want empty on verification failure", stdout.String())
				}
				return
			}

			if err != nil {
				t.Fatalf("release rollback --wait: %v", err)
			}
			if stdout.Len() == 0 {
				t.Fatal("stdout is empty, want successful rollback result")
			}
		})
	}
}

func TestReleaseRollbackWaitFailureReturnsErrorWithEmptyStdout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			_, _ = w.Write([]byte(`{"items":[{"id":"release-previous"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","sequence":7,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: end\ndata: {}\n\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback":
			_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Failed","message":"image pull failed"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID:
			_, _ = w.Write([]byte(`{"id":"` + rollbackTestStackID + `","name":"demo","spec":{},"latest_release":{"id":"release-rollback"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx, stdout := rollbackCommandContext(ts.URL)
	cmd := newReleaseCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"rollback", "release-previous", "--wait"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("failed rollback returned nil")
	}
	if !strings.Contains(err.Error(), "image pull failed") {
		t.Errorf("rollback error = %v, want release failure detail", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failed rollback", stdout.String())
	}
}

func TestReleaseRollbackWaitTimeoutIsNotUserCancellation(t *testing.T) {
	ts := rollbackBlockingServer(t, nil)
	defer ts.Close()

	ctx, stdout := rollbackCommandContext(ts.URL)
	cmd := newReleaseCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"rollback", "release-previous", "--wait", "--timeout", "20ms"})

	started := time.Now()
	err := cmd.Execute()
	if err == nil {
		t.Fatal("release rollback --wait returned nil, want timeout error")
	}
	if err == clierrors.ErrUserCanceled {
		t.Fatalf("deadline reported as user cancellation: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("rollback timeout took %s, want under 1s", elapsed)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no partial result", stdout.String())
	}
}

func TestReleaseRollbackWaitTimeoutBeforeStreamHeadersUsesTimeoutContract(t *testing.T) {
	streamStarted := make(chan struct{})
	ts := rollbackUnflushedStreamServer(t, streamStarted)
	defer ts.Close()

	ctx, stdout := rollbackCommandContext(ts.URL)
	cmd := newReleaseCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"rollback", "release-previous", "--wait", "--timeout", "20ms"})

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

func TestReleaseRollbackWaitParentCancellationIsUserCancellation(t *testing.T) {
	streamStarted := make(chan struct{})
	ts := rollbackBlockingServer(t, streamStarted)
	defer ts.Close()

	ctx, stdout := rollbackCommandContext(ts.URL)
	parent, cancel := context.WithCancel(context.Background())
	cmd := newReleaseCmd()
	cmd.SetContext(parent)
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"rollback", "release-previous", "--wait", "--timeout", "1m"})

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.Execute() }()
	select {
	case <-streamStarted:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("rollback event stream did not start")
	}

	select {
	case err := <-errCh:
		if err != clierrors.ErrUserCanceled {
			t.Fatalf("cancellation error = %v, want ErrUserCanceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("rollback command did not return after cancellation")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no partial result", stdout.String())
	}
}

// Parent cancellation remains an interrupt even after the terminal release has
// been observed and the command is fetching the converged release's live
// status. No structured rollback result may be emitted first.
func TestReleaseRollbackWaitParentCancellationDuringLiveStatusIsUserCancellation(t *testing.T) {
	liveFetchStarted := make(chan struct{})
	var releaseReads int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			_, _ = w.Write([]byte(`{"items":[{"id":"release-previous"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","sequence":7,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: end\ndata: {}\n\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback":
			releaseReads++
			if releaseReads == 1 {
				_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Released"}`))
				return
			}
			close(liveFetchStarted)
			<-r.Context().Done()
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID:
			_, _ = w.Write([]byte(`{"id":"` + rollbackTestStackID + `","name":"demo","spec":{},"converged_release":{"id":"release-rollback"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx, stdout := rollbackCommandContext(ts.URL)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := newReleaseCmd()
	cmd.SetContext(parent)
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"rollback", "release-previous", "--wait", "--timeout", "1m"})

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.Execute() }()
	select {
	case <-liveFetchStarted:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("rollback live-status fetch did not start")
	}
	select {
	case err := <-errCh:
		if err != clierrors.ErrUserCanceled {
			t.Fatalf("cancellation error = %v, want ErrUserCanceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("rollback command did not return after cancellation")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no partial result", stdout.String())
	}
}

func TestReleaseEventsFollowYAMLFailsValidationBeforeAPIRequest(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
		CurrentStack:   rollbackTestStackID,
	}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatYAML, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newReleaseCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"events", "22222222-2222-2222-2222-222222222222", "--follow"})
	err := cmd.Execute()
	if clierrors.ExitCodeFrom(err) != clierrors.ExitValidation {
		t.Fatalf("exit code = %d (%v), want validation", clierrors.ExitCodeFrom(err), err)
	}
	if requests != 0 {
		t.Errorf("API requests = %d, want 0", requests)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestReleaseEventsFollowYAMLValidatesBeforeScopeDiscovery(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := &config.Config{
		ServerURL:    ts.URL,
		AccessToken:  "sdm_test",
		CurrentStack: rollbackTestStackID,
	}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatYAML, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newReleaseCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"events", "22222222-2222-2222-2222-222222222222", "--follow"})
	err := cmd.Execute()
	if clierrors.ExitCodeFrom(err) != clierrors.ExitValidation {
		t.Fatalf("exit code = %d (%v), want validation", clierrors.ExitCodeFrom(err), err)
	}
	if requests != 0 {
		t.Errorf("scope discovery API requests = %d, want 0", requests)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestReleaseEventsNonFollowYAMLRemainsAllowed(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/22222222-2222-2222-2222-222222222222/events" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"total":0}`))
	}))
	defer ts.Close()

	cfg := &config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
		CurrentStack:   rollbackTestStackID,
	}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatYAML, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout

	cmd := newReleaseCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"events", "22222222-2222-2222-2222-222222222222"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("non-follow YAML events: %v", err)
	}
	if requests != 1 {
		t.Errorf("API requests = %d, want 1", requests)
	}
	if stdout.Len() == 0 {
		t.Fatal("non-follow YAML output is empty")
	}
}

// StreamReleaseEvents closes its channel when the caller's context ends. The
// follow command must turn that otherwise-silent close into the CLI's standard
// cancellation contract instead of reporting success.
func TestReleaseEventsFollowParentCancellationIsUserCancellation(t *testing.T) {
	streamStarted := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			_, _ = w.Write([]byte(`{"items":[{"id":"release-follow"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-follow/events/stream":
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

	cctx, stdout := rollbackCommandContext(ts.URL)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := newReleaseCmd()
	cmd.SetContext(parent)
	cmdutil.SetContext(cmd, cctx)
	cmd.SetArgs([]string{"events", "release-follow", "--follow"})

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.Execute() }()
	select {
	case <-streamStarted:
		// Give the client a brief scheduling window to return its event channel
		// before cancelling the parent. This targets the close-after-connect
		// path rather than the separate initial-request cancellation path.
		time.Sleep(20 * time.Millisecond)
		cancel()
	case <-time.After(time.Second):
		t.Fatal("release events stream did not start")
	}

	select {
	case err := <-errCh:
		if err != clierrors.ErrUserCanceled {
			t.Fatalf("cancellation error = %v, want ErrUserCanceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("release events command did not return after cancellation")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no partial event output", stdout.String())
	}
}

func rollbackCommandContext(serverURL string) (*cmdutil.CommandContext, *bytes.Buffer) {
	cfg := &config.Config{
		ServerURL:      serverURL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
		CurrentStack:   rollbackTestStackID,
	}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	stdout := &bytes.Buffer{}
	ctx.Formatter.Writer = stdout
	return ctx, stdout
}

func rollbackObservationServer(t *testing.T, stackResponse, liveResponse string) *httptest.Server {
	t.Helper()
	var releaseReads int
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			_, _ = w.Write([]byte(`{"items":[{"id":"release-previous"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","sequence":7,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: end\ndata: {}\n\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID:
			_, _ = w.Write([]byte(stackResponse))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback":
			releaseReads++
			if releaseReads == 1 {
				_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","state":"Released"}`))
				return
			}
			_, _ = w.Write([]byte(liveResponse))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-other":
			_, _ = w.Write([]byte(liveResponse))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func rollbackBlockingServer(t *testing.T, streamStarted chan<- struct{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			_, _ = w.Write([]byte(`{"items":[{"id":"release-previous"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","sequence":7,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			if streamStarted != nil {
				close(streamStarted)
			}
			<-r.Context().Done()
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func rollbackUnflushedStreamServer(t *testing.T, streamStarted chan<- struct{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			_, _ = w.Write([]byte(`{"items":[{"id":"release-previous"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"release-rollback","stack_id":"` + rollbackTestStackID + `","sequence":7,"state":"Pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/stacks/"+rollbackTestStackID+"/releases/release-rollback/events/stream":
			// Deliberately do not write or flush headers. This holds
			// http.Client.Do inside StreamReleaseEvents until the deadline.
			close(streamStarted)
			<-r.Context().Done()
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestEventDataJSONStaysValid(t *testing.T) {
	for _, data := range []string{`{"state":"Released"}`, "plain text \"quoted\"", ""} {
		line := `{"event":"message","data":` + eventDataJSON(data) + `}`
		if !json.Valid([]byte(line)) {
			t.Errorf("invalid NDJSON line for payload %q: %s", data, line)
		}
	}
}

func TestRedactSecret(t *testing.T) {
	if got := redactSecret("sdm_abcdefghijklmnop"); got != "<redacted>" {
		t.Errorf("long token: got %q", got)
	}
	if got := redactSecret("short"); got != "<redacted>" {
		t.Errorf("short token: got %q", got)
	}
	if got := redactSecret(""); got != "" {
		t.Errorf("empty token: got %q", got)
	}
}
