package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
)

func apiTestCommand(serverURL string, format output.Format) (*bytes.Buffer, *bytes.Buffer, *cmdutil.CommandContext) {
	ctx := cmdutil.NewCommandContext(&config.Config{ServerURL: serverURL, AccessToken: "sdm_test"}, format, slog.LevelError)
	return &bytes.Buffer{}, &bytes.Buffer{}, ctx
}

// StringSlice treats a comma as CSV syntax, but HTTP header values commonly
// contain commas and must arrive at the server byte-for-byte.
func TestAPICommandPreservesCommaHeaderValuesAndRepeatedHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Values("Accept"), []string{"application/json, text/plain"}; strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("Accept = %q, want %q", got, want)
		}
		if got, want := r.Header.Values("X-Trace"), []string{"one", "two"}; strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("X-Trace = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if _, err := executeAPITestCommand(t, server.URL, output.FormatJSON,
		"/api/v1/test", "--header", "Accept: application/json, text/plain", "--header", "X-Trace: one", "--header", "X-Trace: two"); err != nil {
		t.Fatalf("api command: %v", err)
	}
}

func executeAPITestCommand(t *testing.T, serverURL string, format output.Format, args ...string) (string, error) {
	t.Helper()
	stdout, stderr, ctx := apiTestCommand(serverURL, format)
	ctx.Formatter.Writer = stdout
	cmd := newAPICmd()
	cmd.SilenceUsage = true
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), err
}

// Reintroducing RequireAuth here would discover organization/project before
// the arbitrary API call, which breaks tokens scoped only to that endpoint.
func TestAPICommandDefaultsToAuthenticatedGETWithoutScopeDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodGet; got != want {
			t.Errorf("method = %s, want %s", got, want)
		}
		if got, want := r.URL.RequestURI(), "/api/v1/users/current?verbose=true"; got != want {
			t.Errorf("request URI = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer sdm_test"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		if got, want := r.Header.Values("X-Trace"), []string{"one", "two"}; strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("X-Trace = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"user-1"}`))
	}))
	defer server.Close()

	stdout, err := executeAPITestCommand(t, server.URL, output.FormatJSON,
		"/api/v1/users/current?verbose=true", "--header", "X-Trace: one", "--header", "X-Trace: two")
	if err != nil {
		t.Fatalf("api command: %v", err)
	}
	if got, want := stdout, `{"id":"user-1"}`; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

// Dropping a supported verb silently changes the requested API operation.
func TestAPICommandSupportsAllContractMethods(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Method; got != method {
					t.Errorf("method = %q, want %q", got, method)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			args := []string{"/api/v1/test", "--method", method}
			if method != http.MethodGet && method != http.MethodHead {
				args = append(args, "--yes")
			}
			stdout, err := executeAPITestCommand(t, server.URL, output.FormatJSON, args...)
			if err != nil {
				t.Fatalf("api command: %v", err)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty for 204", stdout)
			}
		})
	}
}

func TestAPICommandRejectsUnsupportedMethod(t *testing.T) {
	stdout, err := executeAPITestCommand(t, "http://127.0.0.1:1", output.FormatJSON, "/api/v1/test", "--method", "OPTIONS")
	if err == nil {
		t.Fatal("api command error = nil, want method validation error")
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// Choosing both data sources is ambiguous and must fail before a request is
// sent; a JSON body also needs the default content type for the API contract.
func TestAPICommandHandlesJSONDataSources(t *testing.T) {
	t.Run("data sets JSON content type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got, want := r.Header.Get("Content-Type"), "application/json"; got != want {
				t.Errorf("Content-Type = %q, want %q", got, want)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(body), `{"name":"demo"}`; got != want {
				t.Errorf("body = %q, want %q", got, want)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		if _, err := executeAPITestCommand(t, server.URL, output.FormatJSON, "/api/v1/test", "-X", "POST", "--data", `{"name":"demo"}`, "--yes"); err != nil {
			t.Fatalf("api command: %v", err)
		}
	})

	t.Run("data file sends its JSON body", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "request.json")
		if err := os.WriteFile(path, []byte(`{"name":"file"}`), 0600); err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(body), `{"name":"file"}`; got != want {
				t.Errorf("body = %q, want %q", got, want)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		if _, err := executeAPITestCommand(t, server.URL, output.FormatJSON, "/api/v1/test", "-X", "POST", "--data-file", path, "--yes"); err != nil {
			t.Fatalf("api command: %v", err)
		}
	})

	t.Run("data and data file are exclusive", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "request.json")
		if err := os.WriteFile(path, []byte(`{"name":"file"}`), 0600); err != nil {
			t.Fatal(err)
		}
		stdout, err := executeAPITestCommand(t, "http://127.0.0.1:1", output.FormatJSON,
			"/api/v1/test", "--data", `{"name":"inline"}`, "--data-file", path, "--yes")
		if err == nil {
			t.Fatal("api command error = nil, want mutually exclusive data error")
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
	})
}

// Mutating API methods must not turn a non-interactive invocation into an
// accidental write when --yes is absent.
func TestAPICommandMutatingRequestRequiresYesWithoutSending(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	stdout, err := executeAPITestCommand(t, server.URL, output.FormatJSON, "/api/v1/test", "-X", "DELETE")
	if err == nil {
		t.Fatal("api command error = nil, want confirmation error")
	}
	if requests != 0 {
		t.Errorf("requests = %d, want 0", requests)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// Permitting any spelling of these headers lets a user replace CLI-managed
// credentials or routing before the authenticated request is made.
func TestAPICommandRejectsCredentialBearingHeaderOverrides(t *testing.T) {
	for _, header := range []string{
		"aUtHoRiZaTiOn: Bearer replacement",
		"PrOxY-AuThOrIzAtIoN: Basic replacement",
		"hOsT: attacker.test",
		"cOoKiE: session=stolen",
	} {
		t.Run(header, func(t *testing.T) {
			stdout, err := executeAPITestCommand(t, "http://127.0.0.1:1", output.FormatJSON,
				"/api/v1/test", "--header", header)
			if err == nil {
				t.Fatal("api command error = nil, want protected-header rejection")
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
		})
	}
}

func TestAPICommandSuccessfulOutputHandling(t *testing.T) {
	t.Run("204 writes no stdout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		stdout, err := executeAPITestCommand(t, server.URL, output.FormatJSON, "/api/v1/test")
		if err != nil {
			t.Fatalf("api command: %v", err)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
	})

	t.Run("yaml converts JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"voyager","enabled":true}`))
		}))
		defer server.Close()

		stdout, err := executeAPITestCommand(t, server.URL, output.FormatYAML, "/api/v1/test")
		if err != nil {
			t.Fatalf("api command: %v", err)
		}
		if got, want := stdout, "enabled: true\nname: voyager\n"; got != want {
			t.Errorf("stdout = %q, want %q", got, want)
		}
	})

	t.Run("yaml preserves large JSON integers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"id":9007199254740993}`))
		}))
		defer server.Close()

		stdout, err := executeAPITestCommand(t, server.URL, output.FormatYAML, "/api/v1/test")
		if err != nil {
			t.Fatalf("api command: %v", err)
		}
		if got, want := stdout, "id: 9007199254740993\n"; got != want {
			t.Errorf("stdout = %q, want %q", got, want)
		}
	})
}

func TestAPICommandRejectsPathsEscapingAPI(t *testing.T) {
	for _, path := range []string{
		"/api/../settings",
		"/api/%2e%2e/settings",
		"/api/%2E%2E%2Fsettings",
		"/api/%252e%252e/settings",
		"/api/%252E%252E%252Fsettings",
	} {
		t.Run(path, func(t *testing.T) {
			if err := validateAPIPath(path); err == nil {
				t.Fatal("validateAPIPath error = nil, want API-prefix escape rejection")
			}
		})
	}
}

func TestAPICommandRejectsBareAPIPath(t *testing.T) {
	if err := validateAPIPath("/api"); err == nil {
		t.Fatal("validateAPIPath error = nil, want bare /api rejection")
	}
}

// Streaming an error response to stdout lets automation mistake a failed
// request for a valid result.
func TestAPICommandDoesNotWriteNon2xxBodyToStdout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"reason":"invalid API request"}`))
	}))
	defer server.Close()

	stdout, err := executeAPITestCommand(t, server.URL, output.FormatJSON, "/api/v1/test")
	if err == nil {
		t.Fatal("api command error = nil, want HTTP error")
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// The configured API token must not be echoed to either stream even if a
// server returns it in a non-2xx reason.
func TestAPICommandDoesNotLeakConfiguredTokenToOutput(t *testing.T) {
	const token = "sdm_secret_token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer "+token; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"reason":"request rejected for ` + token + `"}`))
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(`{"server_url":%q,"access_token":%q}`, server.URL, token)), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STACKDOME_CONFIG", configPath)
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"--output", "json", "api", "/api/v1/test"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want HTTP error")
	}
	if strings.Contains(stdout.String(), token) || strings.Contains(stderr.String(), token) {
		t.Errorf("token leaked: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

// API errors are JSON-decoded by client.WrapError. Redaction must therefore
// happen after decoding, or JSON escape sequences reveal configured secrets.
func TestAPICommandDoesNotLeakJSONEscapedConfiguredSecrets(t *testing.T) {
	const accessToken = "sdm_secret_token"
	const refreshToken = "refresh_secret_token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"reason":"rejected sdm\u005fsecret\u005ftoken and refresh\u005fsecret\u005ftoken"}`))
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(`{"server_url":%q,"access_token":%q,"refresh_token":%q}`, server.URL, accessToken, refreshToken)), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STACKDOME_CONFIG", configPath)
	var stdout, stderr bytes.Buffer
	code := runWithWriters([]string{"--output", "json", "api", "/api/v1/test"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want HTTP error")
	}
	for _, secret := range []string{accessToken, refreshToken} {
		if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
			t.Errorf("configured secret leaked: stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}
}
