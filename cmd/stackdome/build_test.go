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
	"strings"
	"testing"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"github.com/Stackdome/stackdome/pkg/api/openapi"
)

func TestBuildLogsRejectsYAMLStreamOutput(t *testing.T) {
	const stackID = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds":
			_, _ = w.Write([]byte(`{"items":[{"id":"build-1"}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds/build-1/logs":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: end\ndata: {}\n\n"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1", CurrentStack: stackID}, output.FormatYAML, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newBuildLogsCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"build-1"})

	err := cmd.Execute()
	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("build logs YAML error = %T (%v), want validation error", err, err)
	}
	if stdout.Len() != 0 || requests != 0 {
		t.Errorf("stdout = %q, requests = %d; want rejection before streaming", stdout.String(), requests)
	}
}

func cond(typ string, t time.Time) openapi.Condition {
	return openapi.Condition{Type: &typ, LastTransitionTime: &t}
}

func buildWith(state string, conds []openapi.Condition) openapi.ImageBuild {
	b := openapi.ImageBuild{Status: &openapi.ImageBuildStatus{Conditions: conds}}
	if state != "" {
		b.Status.State = &state
	}
	return b
}

func TestBuildReferenceUsesCommitAndUniqueBuildSuffix(t *testing.T) {
	commit := "7bc067e829eb9380539878b72d8b64ac017b487a"
	builds := []struct {
		name  string
		build openapi.ImageBuild
		want  string
	}{
		{
			name: "git build",
			build: openapi.ImageBuild{
				Id: openapi.PtrString("api-server-7bc067e829eb9380539878b72d8b64ac017b487a-fe0e849a"),
				SourceRevision: openapi.BuildSourceRevision{
					GitRepoRevision: &openapi.GitRepoRevision{Commit: &commit},
				},
			},
			want: "7bc067e-fe0e849a",
		},
		{
			name: "rebuild of same commit",
			build: openapi.ImageBuild{
				Id: openapi.PtrString("api-server-7bc067e829eb9380539878b72d8b64ac017b487a-a2a97d0d"),
				SourceRevision: openapi.BuildSourceRevision{
					GitRepoRevision: &openapi.GitRepoRevision{Commit: &commit},
				},
			},
			want: "7bc067e-a2a97d0d",
		},
		{
			name: "volume build",
			build: openapi.ImageBuild{
				Id: openapi.PtrString("worker-volume-source-11223344"),
				SourceRevision: openapi.BuildSourceRevision{
					VolumeSourceRevision: &openapi.BuildSourceRevisionVolumeSourceRevision{},
				},
			},
			want: "volume-11223344",
		},
	}

	for _, tt := range builds {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildReference(tt.build); got != tt.want {
				t.Fatalf("buildReference() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildListShowsCopyableBuildReferences(t *testing.T) {
	const stackID = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/organizations/org-1/projects/proj-1/stacks/"+stackID+"/builds" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"items":[
			{"id":"api-server-a214e9a114b01a17c7c4ad576543810eb3a4f421-3e794668","stack_resource_id":"resource-1","stack_resource_name":"api-server","source_revision":{"git_repo_revision":{"branch":"feat/alpha-observability-p0","commit":"a214e9a114b01a17c7c4ad576543810eb3a4f421"}},"build_context":{},"image_repo":"example.invalid/api","status":{"state":"Success"}},
			{"id":"api-server-7bc067e829eb9380539878b72d8b64ac017b487a-fe0e849a","stack_resource_id":"resource-1","stack_resource_name":"api-server","source_revision":{"git_repo_revision":{"branch":"feat/alpha-observability-p0","commit":"7bc067e829eb9380539878b72d8b64ac017b487a"}},"build_context":{},"image_repo":"example.invalid/api","status":{"state":"Success"}}
		]}`))
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
		CurrentStack:   stackID,
	}, output.FormatTable, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newBuildListCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("build list: %v", err)
	}

	for _, want := range []string{"BUILD", "a214e9a-3e794668", "7bc067e-fe0e849a"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("build list output omitted %q:\n%s", want, stdout.String())
		}
	}
}

func TestBuildInfoAcceptsDisplayedBuildReference(t *testing.T) {
	const (
		stackID = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
		buildID = "api-server-7bc067e829eb9380539878b72d8b64ac017b487a-fe0e849a"
	)
	buildJSON := `{"id":"` + buildID + `","stack_id":"` + stackID + `","stack_resource_id":"resource-1","stack_resource_name":"api-server","source_revision":{"git_repo_revision":{"branch":"feat/alpha-observability-p0","commit":"7bc067e829eb9380539878b72d8b64ac017b487a"}},"build_context":{},"image_repo":"example.invalid/api","status":{"state":"Success"}}`
	var gotInfoPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds":
			_, _ = w.Write([]byte(`{"items":[` + buildJSON + `]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds/" + buildID:
			gotInfoPath = r.URL.Path
			_, _ = w.Write([]byte(buildJSON))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
		CurrentStack:   stackID,
	}, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newBuildInfoCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"7bc067e-fe0e849a"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("build info by displayed reference: %v", err)
	}
	if !strings.HasSuffix(gotInfoPath, "/builds/"+buildID) {
		t.Fatalf("build info path = %q, want full build ID", gotInfoPath)
	}
	var got openapi.ImageBuild
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("build info output is not JSON: %v\n%s", err, stdout.String())
	}
	if got.GetId() != buildID {
		t.Fatalf("build info ID = %q, want %q", got.GetId(), buildID)
	}
}

func TestBuildInfoTableShowsReferenceAndFullID(t *testing.T) {
	const (
		stackID = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
		buildID = "api-server-7bc067e829eb9380539878b72d8b64ac017b487a-fe0e849a"
	)
	buildJSON := `{"id":"` + buildID + `","stack_id":"` + stackID + `","stack_resource_id":"resource-1","stack_resource_name":"api-server","source_revision":{"git_repo_revision":{"branch":"feat/alpha-observability-p0","commit":"7bc067e829eb9380539878b72d8b64ac017b487a"}},"build_context":{},"image_repo":"example.invalid/api","status":{"state":"Success"}}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds":
			_, _ = w.Write([]byte(`{"items":[` + buildJSON + `]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds/" + buildID:
			_, _ = w.Write([]byte(buildJSON))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
		CurrentStack:   stackID,
	}, output.FormatTable, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newBuildInfoCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"7bc067e-fe0e849a"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("build info by displayed reference: %v", err)
	}
	for _, want := range []string{
		"Build:     7bc067e-fe0e849a",
		"ID:        " + buildID,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("build info output omitted %q:\n%s", want, stdout.String())
		}
	}
}

func TestBuildDurationConditionFallback(t *testing.T) {
	base := time.Date(2026, 8, 5, 20, 38, 0, 0, time.UTC)
	done := base.Add(time.Minute + 41*time.Second)

	created := base.Add(-time.Hour)
	updated := created.Add(30 * time.Second)

	tests := []struct {
		name      string
		build     openapi.ImageBuild
		wantStart *time.Time
		wantDur   string
	}{
		{
			name: "model timestamps win",
			build: openapi.ImageBuild{
				CreatedAt: &created,
				UpdatedAt: &updated,
				Status:    &openapi.ImageBuildStatus{Conditions: []openapi.Condition{cond("BuildJobCreated", base)}},
			},
			wantStart: &created,
			wantDur:   "30s",
		},
		{
			name: "conditions present, terminal",
			build: buildWith("Success", []openapi.Condition{
				cond("BuildJobCreated", base),
				cond("Available", done),
			}),
			wantStart: &base,
			wantDur:   "1m41s",
		},
		{
			name: "failed build uses latest condition as end",
			build: buildWith("Failed", []openapi.Condition{
				cond("BuildJobCreated", base),
				cond("BuildJobFailed", done),
			}),
			wantStart: &base,
			wantDur:   "1m41s",
		},
		{
			name: "only created condition, terminal but no end",
			build: buildWith("Success", []openapi.Condition{
				cond("BuildJobCreated", base),
			}),
			wantStart: &base,
			wantDur:   "-",
		},
		{
			name:      "no conditions at all",
			build:     buildWith("Pending", nil),
			wantStart: nil,
			wantDur:   "-",
		},
		{
			name:      "no status at all",
			build:     openapi.ImageBuild{},
			wantStart: nil,
			wantDur:   "-",
		},
		{
			name: "no named condition falls back to earliest",
			build: buildWith("Success", []openapi.Condition{
				cond("Reconciling", done),
				cond("Initialized", base),
			}),
			wantStart: &base,
			wantDur:   "1m41s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildStartTime(tt.build)
			switch {
			case tt.wantStart == nil && got != nil:
				t.Fatalf("start = %v, want nil", got)
			case tt.wantStart != nil && (got == nil || !got.Equal(*tt.wantStart)):
				t.Fatalf("start = %v, want %v", got, *tt.wantStart)
			}
			if d := buildDuration(tt.build); d != tt.wantDur {
				t.Errorf("duration = %q, want %q", d, tt.wantDur)
			}
			if tt.wantStart == nil && buildStarted(tt.build) != "-" {
				t.Errorf("started = %q, want %q", buildStarted(tt.build), "-")
			}
		})
	}
}

func TestBuildDurationInProgressShowsElapsed(t *testing.T) {
	b := buildWith("Building", []openapi.Condition{
		// mid-bucket offset so rounding can't tip either way while the test runs
		cond("BuildJobCreated", time.Now().Add(-90*time.Second-250*time.Millisecond)),
	})
	if d := buildDuration(b); d != "1m30s" {
		t.Errorf("duration = %q, want %q", d, "1m30s")
	}
}

// Build logs share the runtime log contract: each JSON stream item is a
// compact NDJSON event with its JSON data decoded exactly once.
func TestBuildLogsJSONWritesDecodedNDJSONEvent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/builds":
			_, _ = w.Write([]byte(`{"items":[{"id":"build-1"}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/builds/build-1/logs":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: build\ndata: {\"phase\":\"compile\"}\n\nevent: build\ndata: {\"phase\":\"package\"}\n\nevent: end\ndata: {}\n\n"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	cmd := newBuildLogsCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--stack", "app", "build-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("build logs: %v", err)
	}

	if bytes.Contains(stdout.Bytes(), []byte("\n  ")) {
		t.Fatalf("build log stream is indented instead of compact: %q", stdout.String())
	}
	var lines []struct {
		Event string `json:"event"`
		Data  struct {
			Phase string `json:"phase"`
		} `json:"data"`
	}
	scanner := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	for scanner.Scan() {
		var line struct {
			Event string `json:"event"`
			Data  struct {
				Phase string `json:"phase"`
			} `json:"data"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("build log line is not independent JSON: %v\nline: %s", err, scanner.Text())
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan build log output: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("decoded %d lines, want 2: %s", len(lines), stdout.String())
	}
	if lines[0].Event != "build" || lines[0].Data.Phase != "compile" || lines[1].Event != "build" || lines[1].Data.Phase != "package" {
		t.Errorf("lines = %#v, want compile and package build events", lines)
	}
}

func TestBuildLogsPrunedErrorUsesCopyableReferenceWithoutClusterInternals(t *testing.T) {
	const (
		stackID  = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
		buildID  = "api-server-7bc067e829eb9380539878b72d8b64ac017b487a-fe0e8490"
		buildRef = "7bc067e-fe0e8490"
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds":
			_, _ = w.Write([]byte(`{"items":[{"id":"` + buildID + `","stack_resource_id":"resource-1","stack_resource_name":"api-server","source_revision":{"git_repo_revision":{"commit":"7bc067e829eb9380539878b72d8b64ac017b487a"}},"build_context":{},"image_repo":"example.invalid/api","status":{"state":"Success"}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds/" + buildID + "/logs":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"reason":"no logs available for build ` + buildID + `: no build pod found for job api-server-c4dfdf78-build: the build has not started yet or its logs have been pruned"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
		CurrentStack:   stackID,
	}, output.FormatTable, slog.LevelError)
	cmd := newBuildLogsCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"-f", buildRef})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("build logs error = nil, want pruned-logs error")
	}
	got := clierrors.UserMessage(err)
	want := `Logs for build "7bc067e-fe0e8490" are no longer available; they were pruned after the build completed.`
	if got != want {
		t.Fatalf("user message = %q, want %q", got, want)
	}
	for _, leaked := range []string{buildID, "build pod", "api-server-c4dfdf78-build"} {
		if strings.Contains(got, leaked) {
			t.Errorf("user message leaks cluster detail %q: %s", leaked, got)
		}
	}
	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != "NOT_FOUND" || cliErr.ExitCode != clierrors.ExitNotFound {
		t.Errorf("error = %#v, want NOT_FOUND with exit code %d", err, clierrors.ExitNotFound)
	}
}

func TestBuildLogsUnavailableForPendingBuildDoesNotClaimPruning(t *testing.T) {
	const (
		stackID  = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
		buildID  = "api-server-7bc067e829eb9380539878b72d8b64ac017b487a-fe0e8490"
		buildRef = "7bc067e-fe0e8490"
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds":
			_, _ = w.Write([]byte(`{"items":[{"id":"` + buildID + `","stack_resource_id":"resource-1","stack_resource_name":"api-server","source_revision":{"git_repo_revision":{"commit":"7bc067e829eb9380539878b72d8b64ac017b487a"}},"build_context":{},"image_repo":"example.invalid/api","status":{"state":"Pending"}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds/" + buildID + "/logs":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"reason":"no logs available for build ` + buildID + `: no build pod found for job api-server-c4dfdf78-build: the build has not started yet or its logs have been pruned"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{
		ServerURL:      ts.URL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
		CurrentStack:   stackID,
	}, output.FormatTable, slog.LevelError)
	cmd := newBuildLogsCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"-f", buildRef})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("build logs error = nil, want unavailable-logs error")
	}
	got := clierrors.UserMessage(err)
	want := `Logs for build "7bc067e-fe0e8490" are not available yet; the build has not started.`
	if got != want {
		t.Fatalf("user message = %q, want %q", got, want)
	}
	if strings.Contains(strings.ToLower(got), "pruned") {
		t.Fatalf("pending-build message incorrectly claims pruning: %s", got)
	}
}

// Build-stream errors follow the same root-only JSON error contract as
// runtime logs; prose from the callback would corrupt the document.
func TestBuildLogsJSONServerErrorIsSingleRootDocument(t *testing.T) {
	if os.Getenv("STACKDOME_TEST_BUILD_LOG_ERROR_HELPER") == "1" {
		os.Exit(runWithWriters([]string{"build", "logs", "build-1", "--stack", "app", "-o", "json"}, os.Stdout, os.Stderr))
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/builds":
			_, _ = w.Write([]byte(`{"items":[{"id":"build-1"}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/builds/build-1/logs":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: error\ndata: build stream failed\n\n"))
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

	child := exec.Command(os.Args[0], "-test.run=^TestBuildLogsJSONServerErrorIsSingleRootDocument$")
	child.Env = append(os.Environ(), "STACKDOME_TEST_BUILD_LOG_ERROR_HELPER=1", "STACKDOME_CONFIG="+configPath)
	var stdout, stderr bytes.Buffer
	child.Stdout = &stdout
	child.Stderr = &stderr
	if err := child.Run(); err == nil {
		t.Fatal("build logs process succeeded, want server stream failure")
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
	if got.Error != "build stream failed" || got.ExitCode == 0 {
		t.Errorf("error document = %#v, want build stream failure", got)
	}
}

// The build-log stream shares the runtime log interruption contract: once the
// parent context is cancelled after parsing begins, the command returns the
// user-cancelled sentinel rather than the transport read error.
func TestBuildLogsCancellationAfterStreamStartsIsUserCancellation(t *testing.T) {
	const stackID = "b02262ac-8e6e-45cd-b18e-acb5d3f97ce4"
	started := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds":
			_, _ = w.Write([]byte(`{"items":[{"id":"build-1"}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/" + stackID + "/builds/build-1/logs":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			close(started)
			<-r.Context().Done()
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1", CurrentStack: stackID}, output.FormatJSON, slog.LevelError)
	var stdout bytes.Buffer
	ctx.Formatter.Writer = &stdout
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := newBuildLogsCmd()
	cmd.SetContext(parent)
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"build-1"})

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.Execute() }()
	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("build log stream did not start")
	}
	select {
	case err := <-errCh:
		if err != clierrors.ErrUserCanceled {
			t.Fatalf("cancellation error = %v, want ErrUserCanceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("build logs command did not return after cancellation")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
