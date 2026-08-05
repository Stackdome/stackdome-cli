package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("STACKDOME_TOKEN", "sdm_abc")
	t.Setenv("STACKDOME_URL", "https://hub.example")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AccessToken != "sdm_abc" {
		t.Errorf("AccessToken = %q, want sdm_abc", cfg.AccessToken)
	}
	if cfg.ServerURL != "https://hub.example" {
		t.Errorf("ServerURL = %q, want https://hub.example", cfg.ServerURL)
	}
	if err := cfg.RequireAuth(); err != nil {
		t.Errorf("RequireAuth with env credentials and no config file: %v", err)
	}
}

// Env credentials are ephemeral: saving unrelated state (e.g. current stack)
// must not leak the env token or URL into the config file.
func TestEnvVarsNotPersisted(t *testing.T) {
	path := writeConfig(t, `{"server_url":"https://file","access_token":"file_tok"}`)
	t.Setenv("STACKDOME_CONFIG", path)
	t.Setenv("STACKDOME_TOKEN", "sdm_abc")
	t.Setenv("STACKDOME_URL", "https://hub.example")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetCurrentStack("web"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sdm_abc") || strings.Contains(string(data), "hub.example") {
		t.Errorf("env values persisted to disk: %s", data)
	}
	if !strings.Contains(string(data), "file_tok") {
		t.Errorf("file token was clobbered: %s", data)
	}
}

func TestLoadFrom_AdoptsLegacyTeamName(t *testing.T) {
	cfg, err := LoadFrom(writeConfig(t, `{"server_url":"https://x","team_name":"acme"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProjectName != "acme" {
		t.Errorf("expected legacy team_name to become project_name, got %q", cfg.ProjectName)
	}
}

func TestLoadFrom_PrefersProjectName(t *testing.T) {
	cfg, err := LoadFrom(writeConfig(t, `{"server_url":"https://x","team_name":"old","project_name":"new"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProjectName != "new" {
		t.Errorf("expected project_name to win, got %q", cfg.ProjectName)
	}
}
