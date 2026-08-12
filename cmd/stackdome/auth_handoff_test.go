package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/config"
)

func TestSignupPrintsInstanceSpecificWebFlowWithoutCallingAPI(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	replaceStdinWithDevNull(t)

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"signup", "--url", server.URL + "/hub/", "--insecure"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if requests != 0 {
		t.Fatalf("HTTP requests = %d, want no signup API request", requests)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want human guidance on stderr only", stdout.String())
	}
	for _, want := range []string{
		server.URL + "/hub/sign-up",
		server.URL + "/hub/settings/api-tokens",
		"Full access",
		"stackdome login --url '" + server.URL + "/hub' --insecure --token <token>",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	for _, unwanted := range []string{"Name:", "Email:", "Password:", "Organisation name:"} {
		if strings.Contains(stderr.String(), unwanted) {
			t.Errorf("signup prompted for credentials: %q", stderr.String())
		}
	}
}

func TestSignupUsesSelectedInstanceAndPrintsStructuredHandoff(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ServerURL = "https://tenant.stackdome.example"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"signup", "-o", "json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty structured success", stderr.String())
	}
	var got struct {
		ServerURL   string `json:"server_url"`
		SignupURL   string `json:"signup_url"`
		APITokenURL string `json:"api_token_url"`
		Login       string `json:"login_command"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got.ServerURL != "https://tenant.stackdome.example" ||
		got.SignupURL != "https://tenant.stackdome.example/sign-up" ||
		got.APITokenURL != "https://tenant.stackdome.example/settings/api-tokens" ||
		got.Login != "stackdome login --url 'https://tenant.stackdome.example' --token <token>" {
		t.Fatalf("handoff = %#v", got)
	}
}

func TestSignupAllowsInsecureFlagWithSelectedHTTPInstance(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ServerURL = "http://selected.stackdome.example"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"signup", "--insecure", "-o", "json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var got struct {
		Login string `json:"login_command"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got.Login != "stackdome login --url 'http://selected.stackdome.example' --insecure --token <token>" {
		t.Fatalf("login command = %q, want selected insecure instance", got.Login)
	}
}

func TestTokenlessLoginExplainsBrowserSetupWithoutPromptingOrCallingAPI(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	replaceStdinWithDevNull(t)

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"login", "--url", server.URL, "--insecure"}, &stdout, &stderr)

	if code != 4 {
		t.Fatalf("exit code = %d, want validation exit 4; stderr: %s", code, stderr.String())
	}
	if requests != 0 {
		t.Fatalf("HTTP requests = %d, want no API request", requests)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{
		server.URL + "/sign-in",
		server.URL + "/sign-up",
		server.URL + "/settings/api-tokens",
		"Full access",
		"stackdome login --url '" + server.URL + "' --insecure --token <token>",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	for _, unwanted := range []string{"Email:", "Password:"} {
		if strings.Contains(stderr.String(), unwanted) {
			t.Errorf("login prompted for credentials: %q", stderr.String())
		}
	}
}

func TestTokenlessLoginUsesSelectedInstance(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ServerURL = "https://selected.stackdome.example"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"login"}, &stdout, &stderr)

	if code != 4 {
		t.Fatalf("exit code = %d, want validation exit 4; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"https://selected.stackdome.example/sign-in",
		"https://selected.stackdome.example/sign-up",
		"https://selected.stackdome.example/settings/api-tokens",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func TestSignupURLOverridesSelectedInstanceAndQuotesShellMetacharacters(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STACKDOME_CONFIG", configPath)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ServerURL = "https://selected.stackdome.example"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{
		"signup", "--url", "https://override.stackdome.example/team'oops", "-o", "json",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var got struct {
		ServerURL string `json:"server_url"`
		Login     string `json:"login_command"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got.ServerURL != "https://override.stackdome.example/team'oops" {
		t.Fatalf("server URL = %q, want explicit override", got.ServerURL)
	}
	if got.Login != "stackdome login --url 'https://override.stackdome.example/team'\"'\"'oops' --token <token>" {
		t.Fatalf("login command = %q, want shell-safe URL argument", got.Login)
	}
}

func TestLoginRejectsLegacyCredentialFlags(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{
		"login", "--url", "https://tenant.stackdome.example",
		"--email", "ada@example.com", "--password", "secret",
	}, &stdout, &stderr)

	if code != 4 {
		t.Fatalf("exit code = %d, want validation exit 4; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown flag") {
		t.Fatalf("stderr = %q, want removed credential flag error", stderr.String())
	}
}

func TestSignupRejectsLegacyCredentialFlags(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{
		"signup", "--url", "https://tenant.stackdome.example",
		"--name", "Ada", "--email", "ada@example.com", "--password", "secret", "--org", "example",
	}, &stdout, &stderr)

	if code != 4 {
		t.Fatalf("exit code = %d, want validation exit 4; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown flag") {
		t.Fatalf("stderr = %q, want removed credential flag error", stderr.String())
	}
}

func replaceStdinWithDevNull(t *testing.T) {
	t.Helper()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdin
	os.Stdin = devNull
	t.Cleanup(func() {
		os.Stdin = original
		_ = devNull.Close()
	})
}
