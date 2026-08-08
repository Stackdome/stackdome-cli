package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/config"
	"github.com/spf13/cobra"
)

const postgresAddonJSON = `{"id":"pg-1","name":"demo","spec":{"version":{"major":16},"instances":{"count":1},"storage":{"size":"10Gi"},"databases":[{"name":"demo"}]},"status":{"state":"Pending"}}`

func TestRootRegistersPostgresShortcutWithSameHelpAndCreateFlags(t *testing.T) {
	root := newRootCmd()
	shortcut, _, err := root.Find([]string{"postgres"})
	if err != nil || shortcut == root || shortcut.CommandPath() != "stackdome postgres" {
		t.Fatalf("find top-level postgres = %v, %v; want registered shortcut", shortcut, err)
	}
	legacy, _, err := root.Find([]string{"addon", "postgres"})
	if err != nil || legacy == root || legacy.CommandPath() != "stackdome addon postgres" {
		t.Fatalf("find legacy postgres = %v, %v; want retained path", legacy, err)
	}

	for _, command := range []*cobra.Command{shortcut, legacy} {
		create, _, err := command.Find([]string{"create"})
		if err != nil {
			t.Fatalf("find %s create: %v", command.CommandPath(), err)
		}
		for _, flag := range []string{"database", "superuser", "version", "instances", "storage", "wait", "timeout"} {
			if create.Flags().Lookup(flag) == nil {
				t.Errorf("%s create missing --%s", command.CommandPath(), flag)
			}
		}
	}

	for _, args := range [][]string{{"postgres", "--help"}, {"addon", "postgres", "--help"}} {
		var stdout, stderr bytes.Buffer
		code := runWithWriters(args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%v help exit = %d, stderr = %s", args, code, stderr.String())
		}
		for _, subcommand := range []string{"create", "list", "info", "delete", "credentials", "backup", "backups"} {
			if !strings.Contains(stdout.String(), subcommand) {
				t.Errorf("%v help omitted %q:\n%s", args, subcommand, stdout.String())
			}
		}
	}
}

func TestPostgresShortcutAndLegacyPathsRouteCreateAndListEquivalently(t *testing.T) {
	type request struct {
		method string
		path   string
		body   string
	}
	var (
		mu       sync.Mutex
		requests []request
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, request{method: r.Method, path: r.URL.Path, body: string(body)})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			_, _ = w.Write([]byte(postgresAddonJSON))
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"items":[` + postgresAddonJSON + `]}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	configurePostgresCLI(t, server.URL)

	paths := [][]string{{"postgres"}, {"addon", "postgres"}}
	var createOutputs, listOutputs []string
	for _, path := range paths {
		createArgs := append(append([]string{}, path...), "create", "demo", "--database", "demo", "--version", "16", "--instances", "1", "--storage", "10Gi", "-o", "json")
		stdout, stderr, code := runPostgresCLI(createArgs)
		if code != 0 {
			t.Fatalf("%v exit = %d, stderr = %s", createArgs, code, stderr)
		}
		createOutputs = append(createOutputs, stdout)

		listArgs := append(append([]string{}, path...), "list", "-o", "json")
		stdout, stderr, code = runPostgresCLI(listArgs)
		if code != 0 {
			t.Fatalf("%v exit = %d, stderr = %s", listArgs, code, stderr)
		}
		listOutputs = append(listOutputs, stdout)
	}
	if createOutputs[0] != createOutputs[1] {
		t.Errorf("create outputs differ:\nshortcut: %s\nlegacy: %s", createOutputs[0], createOutputs[1])
	}
	if listOutputs[0] != listOutputs[1] {
		t.Errorf("list outputs differ:\nshortcut: %s\nlegacy: %s", listOutputs[0], listOutputs[1])
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 4 {
		t.Fatalf("requests = %#v, want four", requests)
	}
	const endpoint = "/api/v1/organizations/org-1/projects/proj-1/addons/postgres"
	for i, got := range requests {
		wantMethod := http.MethodPost
		if i%2 == 1 {
			wantMethod = http.MethodGet
		}
		if got.method != wantMethod || got.path != endpoint {
			t.Errorf("request %d = %s %s, want %s %s", i, got.method, got.path, wantMethod, endpoint)
		}
	}
	if requests[0].body != requests[2].body {
		t.Errorf("create request bodies differ:\nshortcut: %s\nlegacy: %s", requests[0].body, requests[2].body)
	}
}

func TestPostgresCreateWaitSucceedsForHealthyTerminalStatesOnBothPaths(t *testing.T) {
	tests := []struct {
		name  string
		path  []string
		state string
	}{
		{name: "shortcut ready", path: []string{"postgres"}, state: "Ready"},
		{name: "legacy running", path: []string{"addon", "postgres"}, state: "Running"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gets int
			server := postgresWaitServer(t, func(w http.ResponseWriter, r *http.Request) {
				gets++
				_, _ = w.Write([]byte(postgresAddonWithStatus(tt.state, "")))
			})
			defer server.Close()
			configurePostgresCLI(t, server.URL)

			args := append(append([]string{}, tt.path...), "create", "demo", "--wait", "--timeout", "1s", "-o", "json")
			stdout, stderr, code := runPostgresCLI(args)
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr)
			}
			if gets != 1 {
				t.Errorf("addon GET count = %d, want 1", gets)
			}
			var result struct {
				Status struct {
					State string `json:"state"`
				} `json:"status"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("stdout is not addon JSON: %v\n%s", err, stdout)
			}
			if result.Status.State != tt.state {
				t.Errorf("state = %q, want %q", result.Status.State, tt.state)
			}
		})
	}
}

func TestPostgresCreateWaitFailsForTerminalFailureStates(t *testing.T) {
	for _, state := range []string{"Failed", "Error"} {
		t.Run(state, func(t *testing.T) {
			server := postgresWaitServer(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(postgresAddonWithStatus(state, "database provisioning failed")))
			})
			defer server.Close()
			configurePostgresCLI(t, server.URL)

			stdout, stderr, code := runPostgresCLI([]string{"postgres", "create", "demo", "--wait", "--timeout", "1s", "-o", "json"})
			if code == 0 {
				t.Fatal("exit = 0, want failure")
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, state) || !strings.Contains(stderr, "database provisioning failed") {
				t.Errorf("stderr omitted terminal failure details: %s", stderr)
			}
		})
	}
}

func TestPostgresCreateWaitTimeoutIsBounded(t *testing.T) {
	server := postgresWaitServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(postgresAddonWithStatus("Pending", "")))
	})
	defer server.Close()
	configurePostgresCLI(t, server.URL)

	started := time.Now()
	stdout, stderr, code := runPostgresCLI([]string{"postgres", "create", "demo", "--wait", "--timeout", "20ms", "-o", "json"})
	if code == 0 {
		t.Fatal("exit = 0, want timeout")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("timeout took %s, want under 1s", elapsed)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Timed out waiting for postgres addon") {
		t.Errorf("stderr = %s, want postgres timeout error", stderr)
	}
}

func postgresWaitServer(t *testing.T, get func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/addons/postgres":
			_, _ = w.Write([]byte(postgresAddonJSON))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/organizations/org-1/projects/proj-1/addons/postgres/pg-1":
			get(w, r)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func postgresAddonWithStatus(state, message string) string {
	status := `"state":` + strconv.Quote(state)
	if message != "" {
		status += `,"message":` + strconv.Quote(message)
	}
	return strings.Replace(postgresAddonJSON, `"state":"Pending"`, status, 1)
}

func configurePostgresCLI(t *testing.T, serverURL string) {
	t.Helper()
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	for _, name := range []string{"STACKDOME_URL", "STACKDOME_TOKEN", "STACKDOME_ORG", "STACKDOME_PROJECT"} {
		t.Setenv(name, "")
	}
	cfg := &config.Config{
		ServerURL:      serverURL,
		AccessToken:    "sdm_test",
		OrganizationID: "org-1",
		ProjectName:    "proj-1",
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func runPostgresCLI(args []string) (stdout, stderr string, code int) {
	var out, errOut bytes.Buffer
	code = runWithWriters(args, &out, &errOut)
	return out.String(), errOut.String(), code
}
