package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	"github.com/Stackdome/stackdome-cli/internal/output"
	internalstackfile "github.com/Stackdome/stackdome-cli/internal/stackfile"
	hub "github.com/Stackdome/stackdome/pkg/stackfile"
)

func TestStackfileSchemaJSONIsDraft7Schema(t *testing.T) {
	cmd := newStackfileCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"schema", "-o", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schema: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &schema); err != nil {
		t.Fatalf("schema output is not JSON: %v\n%s", err, stdout.String())
	}
	if got := schema["$schema"]; got != "http://json-schema.org/draft-07/schema#" {
		t.Fatalf("$schema = %v, want draft-07", got)
	}
}

func TestStackfileSchemaAcceptsOutputFlagWhenRegisteredUnderRoot(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	root := newRootCmd()
	root.AddCommand(newStackfileCmd())
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"get", "stackfile-schema", "-o", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("root stackfile schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &schema); err != nil {
		t.Fatalf("schema output is not JSON: %v\n%s", err, stdout.String())
	}
}

func TestStackfileSchemaOutputFileWritesExactEmbeddedBytes(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	outputPath := filepath.Join(t.TempDir(), "stackfile.schema.json")
	root := newRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"get", "stackfile-schema", "--output-file", outputPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("root stackfile schema --output-file: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty for file output", stdout.String())
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read schema output: %v", err)
	}
	if !bytes.Equal(content, internalstackfile.SchemaJSON) {
		t.Fatalf("schema file differs from embedded bytes: got %d bytes, want %d", len(content), len(internalstackfile.SchemaJSON))
	}
}

func TestStackfileExportWritesCanonicalContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"app","settings":{"release_retention_limit":10,"min_successful_releases":5},"spec":{"stack_resources":[{"name":"web","source":{"image":{"ref":"nginx:alpine"}},"ports":[{"name":"http","number":80,"exposed_to_public":true}]}]}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	cmd := newStackfileCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", "app", "-o", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export: %v", err)
	}

	var exported map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &exported); err != nil {
		t.Fatalf("export output is not JSON: %v\n%s", err, stdout.String())
	}
	if exported["name"] != "app" {
		t.Fatalf("exported name = %v, want app", exported["name"])
	}

	outputPath := filepath.Join(t.TempDir(), "stackfile.yaml")
	cmd = newStackfileCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", "app", "--output-file", outputPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("export to file: %v", err)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	if _, err := hub.Load(content); err != nil {
		t.Fatalf("exported file is not canonical stackfile content: %v\n%s", err, content)
	}
}

func TestStackfileExportRejectsUnsupportedConstructs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"app","spec":{"stack_resources":[{"name":"web","init_spec":{"containers":[]},"source":{"image":{"ref":"nginx:alpine"}}}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	cmd := newStackfileCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", "app"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "init_spec") {
		t.Fatalf("export error = %v, want unsupported init_spec error", err)
	}
}

// A restart request is an operational timestamp, not desired stack state. If
// export treats it as declarative configuration, merely restarting a UI-created
// resource permanently prevents that stack from being edited as a Stackfile.
func TestStackfileExportIgnoresOperationalRestartRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"app","spec":{"stack_resources":[{"name":"web","source":{"image":{"ref":"nginx:alpine"}},"lifecycle_config":{"restart_request_time":"2026-08-08T10:00:00Z"}}]}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	cmd := newStackfileCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", "app", "-o", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export after restart: %v", err)
	}
	var exported map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &exported); err != nil {
		t.Fatalf("export output is not JSON: %v\n%s", err, stdout.String())
	}
	if exported["name"] != "app" {
		t.Fatalf("exported name = %v, want app", exported["name"])
	}
}

// Volume mounts live canonically on StackResource.VolumeMounts. The topology
// connection is a duplicate UI-canvas edge; stale edges must not prevent the
// resource's actual deployment mount from round-tripping through Stackfile.
func TestStackfileExportIgnoresStaleVolumeMountConnection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{
  "id":"stack-1",
  "name":"app",
  "spec":{
    "volumes":[{"name":"data","spec":{"size":"1Gi","access_mode":"ReadWriteOnce"}}],
    "stack_resources":[{
      "name":"web",
      "source":{"image":{"ref":"nginx:alpine"}},
      "volume_mounts":[{"source_volume_name":"data","target_path":"/data"}]
    }],
    "connections":[{
      "kind":"volume_mount",
      "from":{"type":"volume","name":"data"},
      "to":{"type":"stack_resource","name":"web"},
      "config":{"mount_path":"/stale-path"}
    }]
  }
}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	cmd := newStackfileCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", "app", "-o", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export with stale volume edge: %v", err)
	}
	var exported struct {
		Resources map[string]struct {
			Volumes []struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"volumes"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &exported); err != nil {
		t.Fatalf("export output is not JSON: %v\n%s", err, stdout.String())
	}
	mounts := exported.Resources["web"].Volumes
	if len(mounts) != 1 || mounts[0].Name != "data" || mounts[0].Path != "/data" {
		t.Fatalf("exported mounts = %#v, want data mounted at /data", mounts)
	}
}

// Older UI-created stacks can store an expressible mount only as a topology
// edge. Materialize it on the resource before removing the duplicate edge so
// the mount survives the Stackfile round trip.
func TestStackfileExportMaterializesEdgeOnlyVolumeMountConnection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{
  "id":"stack-1",
  "name":"app",
  "spec":{
    "volumes":[{"name":"data","spec":{"size":"1Gi","access_mode":"ReadWriteOnce"}}],
    "stack_resources":[{"name":"web","source":{"image":{"ref":"nginx:alpine"}}}],
    "connections":[{
      "kind":"volume_mount",
      "from":{"type":"volume","name":"data"},
      "to":{"type":"stack_resource","name":"web"},
      "config":{"mount_path":"/data"}
    }]
  }
}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	cmd := newStackfileCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", "app", "-o", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export edge-only volume mount: %v", err)
	}
	var exported struct {
		Resources map[string]struct {
			Volumes []struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"volumes"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &exported); err != nil {
		t.Fatalf("export output is not JSON: %v\n%s", err, stdout.String())
	}
	mounts := exported.Resources["web"].Volumes
	if len(mounts) != 1 || mounts[0].Name != "data" || mounts[0].Path != "/data" {
		t.Fatalf("exported mounts = %#v, want edge materialized as data at /data", mounts)
	}
}

// Edge options Stackfile cannot represent must remain visible to the Hub
// exporter so it fails instead of silently discarding them.
func TestStackfileExportPreservesNonRedundantVolumeMountConnections(t *testing.T) {
	tests := []struct {
		name      string
		stackJSON string
	}{
		{
			name: "edge with sub path",
			stackJSON: `{
  "id":"stack-1",
  "name":"app",
  "spec":{
    "volumes":[{"name":"data","spec":{"size":"1Gi","access_mode":"ReadWriteOnce"}}],
    "stack_resources":[{
      "name":"web",
      "source":{"image":{"ref":"nginx:alpine"}},
      "volume_mounts":[{"source_volume_name":"data","target_path":"/data"}]
    }],
    "connections":[{
      "kind":"volume_mount",
      "from":{"type":"volume","name":"data"},
      "to":{"type":"stack_resource","name":"web"},
      "config":{"mount_path":"/data","sub_path":"nested"}
    }]
  }
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v1/organizations/org-1/projects/proj-1/stacks":
					_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
				case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
					_, _ = w.Write([]byte(tt.stackJSON))
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer ts.Close()

			cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
			ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
			cmd := newStackfileCmd()
			cmd.SetContext(context.Background())
			cmdutil.SetContext(cmd, ctx)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"export", "app", "-o", "json"})

			if err := cmd.Execute(); err == nil {
				t.Fatal("export succeeded, want failure rather than silently dropping the volume edge")
			}
		})
	}
}

// The process boundary must retain the safe conversion reason. Otherwise a
// human or agent sees only "Cannot export" and has no path to remediation.
func TestStackfileExportRootErrorNamesUnsupportedConstruct(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"app","spec":{"stack_resources":[{"name":"web","init_spec":{"containers":[]},"source":{"image":{"ref":"nginx:alpine"}}}]}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1", Insecure: true}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"export", "stackfile", "app"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want failure")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `resource "web": init_spec`) {
		t.Fatalf("stderr does not identify unsupported construct:\n%s", stderr.String())
	}
}

// Conversion may fail after copying environment values into the candidate
// Stackfile. Arbitrary upstream errors can echo those values, so only known
// structural "not expressible" reasons may cross the process boundary.
func TestStackfileExportRootErrorDoesNotLeakEnvironmentValues(t *testing.T) {
	const sensitiveValue = "credential-must-not-appear-in-errors"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{
  "id":"stack-1",
  "name":"app",
  "spec":{
    "stack_resources":[
      {"name":"db","source":{"image":{"ref":"postgres:16"}}},
      {
        "name":"web",
        "source":{"image":{"ref":"nginx:alpine"}},
        "execution_config":{"environment_variables":[{"name":"DATABASE_URL","value":"` + sensitiveValue + `"}]}
      }
    ],
    "connections":[{
      "kind":"env",
      "from":{"type":"stack_resource","name":"db"},
      "to":{"type":"stack_resource","name":"web"},
      "mappings":[{"target":{"type":"env","name":"DATABASE_URL"},"value":{"output":"url"}}]
    }]
  }
}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1", Insecure: true}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"export", "stackfile", "app", "-o", "json"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want failure")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if strings.Contains(stderr.String(), sensitiveValue) {
		t.Fatalf("stderr leaked an environment value: %s", stderr.String())
	}
	var got struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, stderr.String())
	}
	if got.Error != "Cannot export stack as Stackfile" {
		t.Fatalf("error = %q, want generic safe conversion failure", got.Error)
	}
}

func TestStackfileExportRestoresSecretAndPostgresNames(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{
  "id":"stack-1",
  "name":"app",
  "spec":{
    "stack_resources":[{"name":"web","source":{"image":{"ref":"nginx:alpine"}}}],
    "connections":[
      {
        "kind":"env",
        "from":{"type":"secret","id":"sec-1"},
        "to":{"type":"stack_resource","name":"web"},
        "mappings":[{"target":{"type":"env","name":"API_KEY"},"value":{"output":"token"}}]
      },
      {
        "kind":"env",
        "from":{"type":"addon/postgres","id":"pg-1"},
        "to":{"type":"stack_resource","name":"web"},
        "mappings":[{"target":{"type":"env","name":"DATABASE_URL"},"value":{"output":"url"}}]
      }
    ]
  }
}`))
		case "/api/v1/organizations/org-1/projects/proj-1/secrets":
			_, _ = w.Write([]byte(`{"items":[{"id":"sec-1","name":"app-secrets","type":"Generic","data":[{"key":"token","value":"credential-must-not-leak"}]}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/addons/postgres":
			_, _ = w.Write([]byte(`{"items":[{"id":"pg-1","name":"database","spec":{}}]}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	cmd := newStackfileCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", "app", "-o", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export resolved references: %v", err)
	}
	if strings.Contains(stdout.String(), "credential-must-not-leak") {
		t.Fatalf("export leaked a secret credential: %s", stdout.String())
	}

	var exported struct {
		Resources map[string]struct {
			Secrets map[string]map[string]string `json:"secrets"`
			Addons  map[string]struct {
				Type string            `json:"type"`
				Env  map[string]string `json:"env"`
			} `json:"addons"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &exported); err != nil {
		t.Fatalf("export output is not JSON: %v\n%s", err, stdout.String())
	}
	web := exported.Resources["web"]
	if got := web.Secrets["app-secrets"]["API_KEY"]; got != "token" {
		t.Fatalf("secret mapping = %q, want token", got)
	}
	if addon := web.Addons["database"]; addon.Type != "postgres" || addon.Env["DATABASE_URL"] != "{{ url }}" {
		t.Fatalf("postgres addon = %+v, want database/url mapping", addon)
	}
}

func TestStackfileExportFailsWhenSecretIDCannotBeResolved(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/organizations/org-1/projects/proj-1/stacks":
			_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
		case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
			_, _ = w.Write([]byte(`{"id":"stack-1","name":"app","spec":{"stack_resources":[{"name":"web","source":{"image":{"ref":"nginx:alpine"}}}],"connections":[{"kind":"env","from":{"type":"secret","id":"sec-missing"},"to":{"type":"stack_resource","name":"web"},"mappings":[{"target":{"type":"env","name":"API_KEY"},"value":{"output":"token"}}]}]}}`))
		case "/api/v1/organizations/org-1/projects/proj-1/secrets":
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
	ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
	cmd := newStackfileCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", "app"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "sec-missing") {
		t.Fatalf("export error = %v, want unresolved secret ID", err)
	}
}

func TestStackfileExportRejectsEffectiveNonDefaultSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		want     string
	}{
		{
			name:     "release retention",
			settings: `{"release_retention_limit":20,"min_successful_releases":5}`,
			want:     "release_retention_limit",
		},
		{
			name:     "minimum successful releases",
			settings: `{"release_retention_limit":10,"min_successful_releases":2}`,
			want:     "min_successful_releases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v1/organizations/org-1/projects/proj-1/stacks":
					_, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"app","spec":{}}]}`))
				case "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1":
					_, _ = fmt.Fprintf(w, `{"id":"stack-1","name":"app","settings":%s,"spec":{"stack_resources":[{"name":"web","source":{"image":{"ref":"nginx:alpine"}}}]}}`, tt.settings)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer ts.Close()

			cfg := &config.Config{ServerURL: ts.URL, AccessToken: "sdm_test", OrganizationID: "org-1", ProjectName: "proj-1"}
			ctx := cmdutil.NewCommandContext(cfg, output.FormatJSON, slog.LevelError)
			cmd := newStackfileCmd()
			cmd.SetContext(context.Background())
			cmdutil.SetContext(cmd, ctx)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"export", "app"})

			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("export error = %v, want unsupported %s setting", err, tt.want)
			}
		})
	}
}
