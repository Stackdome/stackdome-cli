package cmdutil

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/config"
	clierrors "github.com/stackdome/cli/internal/errors"
)

// With org and project supplied (e.g. via STACKDOME_ORG / STACKDOME_PROJECT)
// no discovery call may be made — a scoped API token is not allowed one. The
// nil Client makes any API call panic, which is the assertion.
func TestResolveScopeSkipsDiscoveryWhenScopeKnown(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", t.TempDir()+"/missing.json")
	t.Setenv("STACKDOME_TOKEN", "sdm_abc")
	t.Setenv("STACKDOME_URL", "https://hub.example")
	t.Setenv("STACKDOME_ORG", "org-123")
	t.Setenv("STACKDOME_PROJECT", "default")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := &CommandContext{Config: cfg}
	if err := resolveScope(ctx, &cobra.Command{}); err != nil {
		t.Fatalf("resolveScope: %v", err)
	}
}

func TestScopeErrorForEnvToken(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", t.TempDir()+"/missing.json")
	t.Setenv("STACKDOME_TOKEN", "sdm_abc")
	t.Setenv("STACKDOME_URL", "https://hub.example")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	orig := clierrors.FromHTTP(403, "")
	got := scopeError(&CommandContext{Config: cfg}, orig)
	msg := clierrors.UserMessage(got)
	if strings.Contains(msg, "stackdome login") {
		t.Errorf("token auth error still suggests login: %q", msg)
	}
	if !strings.Contains(msg, "STACKDOME_ORG") || !strings.Contains(msg, "scope") {
		t.Errorf("error does not explain the token scope problem: %q", msg)
	}
	if clierrors.ExitCodeFrom(got) != clierrors.ExitAuth {
		t.Errorf("exit code = %d, want %d", clierrors.ExitCodeFrom(got), clierrors.ExitAuth)
	}
}

// Session (file) auth keeps the original message — "run stackdome login" is
// the right advice there.
func TestScopeErrorPassesThroughForSessionAuth(t *testing.T) {
	cfg := &config.Config{ServerURL: "https://hub.example", AccessToken: "file_tok"}
	orig := clierrors.FromHTTP(403, "")
	if got := scopeError(&CommandContext{Config: cfg}, orig); got != error(orig) {
		t.Errorf("scopeError rewrote a session-auth error: %v", got)
	}
}
