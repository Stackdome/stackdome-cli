package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/config"
)

func TestConfigSetContextJSONPrintsResult(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"use", "context", "https://example.stackdome.test", "-o", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty structured success", stderr.String())
	}
	var got struct {
		Status        string `json:"status"`
		Resource      string `json:"resource"`
		ServerURL     string `json:"server_url"`
		Authenticated bool   `json:"authenticated"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a JSON context result: %v\nstdout: %s", err, stdout.String())
	}
	if got.Status != "switched" || got.Resource != "context" || got.ServerURL != "https://example.stackdome.test" || got.Authenticated {
		t.Errorf("context result = %#v", got)
	}
}

func TestConfigSetStackRejectsEphemeralEnvTokenContext(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	t.Setenv("STACKDOME_URL", "https://example.stackdome.test")
	t.Setenv("STACKDOME_TOKEN", "sdm_ephemeral")
	t.Setenv("STACKDOME_ORG", "org-1")
	t.Setenv("STACKDOME_PROJECT", "default")

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"use", "stack", "app", "-o", "json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("set-stack succeeded even though an env-token process cannot persist the selection")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--stack") {
		t.Fatalf("stderr = %q, want --stack remediation", stderr.String())
	}
	if _, err := config.LoadFrom(configPath); err != nil {
		t.Fatalf("config should remain readable: %v", err)
	}
}

func TestConfigSetContextRejectsEnvironmentOverrides(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("STACKDOME_URL", "https://old.example")
	t.Setenv("STACKDOME_TOKEN", "sdm_ephemeral")

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"use", "context", "https://new.example", "-o", "json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("set-context succeeded even though environment overrides would keep the old context active")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
	if !strings.Contains(stderr.String(), "STACKDOME_URL") || !strings.Contains(stderr.String(), "STACKDOME_TOKEN") {
		t.Fatalf("stderr = %q, want environment override remediation", stderr.String())
	}
}
