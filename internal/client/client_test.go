package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	clierrors "github.com/stackdome/cli/internal/errors"
)

// refreshServer serves /api/v1/config, rejecting anything but wantToken with a
// 401, and rotates the token pair on /api/v1/auth/refresh.
type refreshServer struct {
	t          *testing.T
	wantToken  string
	authHits   int
	configHits int
	seenAuth   []string
	bodies     []string
	// rejectStatus/rejectBody override the rejection shape (default 401 +
	// "token expired") so tests can exercise the 403-with-token-reason contract.
	rejectStatus int
	rejectBody   string
}

func (s *refreshServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			s.authHits++
			var req struct {
				RefreshToken string `json:"refreshToken"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				s.t.Errorf("refresh body: %v", err)
			}
			if req.RefreshToken != "refresh-old" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			s.wantToken = "access-new"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"token":        "access-new",
				"refreshToken": "refresh-new",
			})
		default:
			s.configHits++
			w.Header().Set("Content-Type", "application/json")
			s.seenAuth = append(s.seenAuth, r.Header.Get("Authorization"))
			if b, _ := io.ReadAll(r.Body); len(b) > 0 {
				s.bodies = append(s.bodies, string(b))
			}
			if r.Header.Get("Authorization") != "Bearer "+s.wantToken {
				status, body := s.rejectStatus, s.rejectBody
				if status == 0 {
					status, body = http.StatusUnauthorized, `{"reason":"token expired"}`
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(body))
				return
			}
			_, _ = w.Write([]byte(`{}`))
		}
	})
}

func TestRefreshOn401RetriesWithNewToken(t *testing.T) {
	srv := &refreshServer{t: t, wantToken: "access-new"}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	var gotAccess, gotRefresh string
	c := New(ts.URL,
		WithTokens("access-old", "refresh-old"),
		WithTokenRefreshCallback(func(a, r string) error {
			gotAccess, gotRefresh = a, r
			return nil
		}),
	)

	_, httpResp, err := c.API().ApiV1ConfigGet(context.Background()).Execute()
	if err != nil {
		t.Fatalf("request failed after refresh: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", httpResp.StatusCode)
	}
	if srv.authHits != 1 {
		t.Errorf("refresh endpoint hit %d times, want 1", srv.authHits)
	}
	if gotAccess != "access-new" || gotRefresh != "refresh-new" {
		t.Errorf("persist callback got (%q, %q), want (access-new, refresh-new)", gotAccess, gotRefresh)
	}
	if c.accessToken != "access-new" || c.refreshToken != "refresh-new" {
		t.Errorf("client tokens = (%q, %q), want (access-new, refresh-new)", c.accessToken, c.refreshToken)
	}
	if len(srv.seenAuth) != 2 || srv.seenAuth[1] != "Bearer access-new" {
		t.Errorf("request auth headers = %v, want retry to carry access-new", srv.seenAuth)
	}
}

// A PAT (`login --token`) has no refresh pair: the 401 must surface untouched,
// as an auth error with exit code 2.
func TestNoRefreshTokenSurfaces401(t *testing.T) {
	srv := &refreshServer{t: t, wantToken: "access-new"}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	refreshed := false
	c := New(ts.URL,
		WithTokens("sdm_pat", ""),
		WithTokenRefreshCallback(func(string, string) error { refreshed = true; return nil }),
	)

	_, httpResp, err := c.API().ApiV1ConfigGet(context.Background()).Execute()
	if err == nil {
		t.Fatal("expected 401 error")
	}
	if httpResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", httpResp.StatusCode)
	}
	if srv.authHits != 0 || refreshed {
		t.Errorf("refresh attempted for PAT: authHits=%d refreshed=%v", srv.authHits, refreshed)
	}

	var cliErr *clierrors.CLIError
	wrapped := WrapError(httpResp, err, "get config")
	if !errors.As(wrapped, &cliErr) || cliErr.ExitCode != clierrors.ExitAuth {
		t.Fatalf("WrapError = %#v, want CLIError with exit code %d", wrapped, clierrors.ExitAuth)
	}
}

// A failure to write the rotated pair to disk must not fail the command: the
// refresh succeeded, so retry with the live token and warn on stderr.
func TestPersistFailureStillRetries(t *testing.T) {
	srv := &refreshServer{t: t, wantToken: "access-new"}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	stderr, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	c := New(ts.URL,
		WithTokens("access-old", "refresh-old"),
		WithTokenRefreshCallback(func(string, string) error {
			return fmt.Errorf("config is read-only")
		}),
	)

	_, httpResp, err := c.API().ApiV1ConfigGet(context.Background()).Execute()
	w.Close()
	warning, _ := io.ReadAll(stderr)

	if err != nil || httpResp.StatusCode != http.StatusOK {
		t.Fatalf("want retry to succeed, got status=%v err=%v", httpResp.StatusCode, err)
	}
	if !strings.Contains(string(warning), "could not save refreshed credentials: config is read-only") {
		t.Errorf("stderr = %q, want a persist warning naming the cause", warning)
	}
}

// An `sdm_`-prefixed PAT carries no refresh semantics even if a stale refresh
// token is lying around in the config file.
func TestPATPrefixSkipsRefresh(t *testing.T) {
	srv := &refreshServer{t: t, wantToken: "access-new"}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	c := New(ts.URL, WithTokens("sdm_pat", "refresh-old"))
	_, httpResp, err := c.API().ApiV1ConfigGet(context.Background()).Execute()
	if err == nil || httpResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got status=%v err=%v", httpResp.StatusCode, err)
	}
	if srv.authHits != 0 {
		t.Errorf("refresh attempted for sdm_ token (%d hits)", srv.authHits)
	}
}

// The retry must replay the original request body, not send an empty one.
func TestRetryReplaysRequestBody(t *testing.T) {
	srv := &refreshServer{t: t, wantToken: "access-new"}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	c := New(ts.URL, WithTokens("access-old", "refresh-old"))

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/stacks", strings.NewReader(`{"name":"web"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer access-old")
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(srv.bodies) != 2 || srv.bodies[1] != `{"name":"web"}` {
		t.Errorf("bodies = %v, want the payload replayed on retry", srv.bodies)
	}
}

// A refresh that itself fails must not recurse: one refresh attempt, original
// 401 surfaced.
func TestFailedRefreshDoesNotRecurse(t *testing.T) {
	srv := &refreshServer{t: t, wantToken: "access-new"}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	c := New(ts.URL, WithTokens("access-old", "refresh-stale"))
	_, httpResp, err := c.API().ApiV1ConfigGet(context.Background()).Execute()
	if err == nil || httpResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got status=%v err=%v", httpResp.StatusCode, err)
	}
	if srv.authHits != 1 {
		t.Errorf("refresh hit %d times, want exactly 1", srv.authHits)
	}
}

// Server contract: an expired or unparseable access token comes back as a 403
// carrying a token reason, so those must still refresh and retry — same set the
// web UI refreshes on (frontend/src/api/client.ts).
func TestRefreshOn403TokenReason(t *testing.T) {
	for name, body := range map[string]string{
		"lowercase":   `{"code":403,"id":"forbidden","kind":"auth","reason":"token expired"}`,
		"capitalized": `{"code":403,"id":"forbidden","kind":"auth","reason":"Token is expired"}`,
		"expired by":  `{"code":403,"id":"forbidden","kind":"auth","reason":"token is expired by 3m0s"}`,
		"parse error": `{"code":403,"id":"forbidden","kind":"auth","reason":"token parse error"}`,
		"items":       `{"code":403,"id":"forbidden","kind":"auth","items":[{"reason":"Token is expired"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := &refreshServer{
				t:            t,
				wantToken:    "access-new",
				rejectStatus: http.StatusForbidden,
				rejectBody:   body,
			}
			ts := httptest.NewServer(srv.handler())
			defer ts.Close()

			c := New(ts.URL, WithTokens("access-old", "refresh-old"))

			_, httpResp, err := c.API().ApiV1ConfigGet(context.Background()).Execute()
			if err != nil {
				t.Fatalf("request failed after refresh: %v", err)
			}
			if httpResp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", httpResp.StatusCode)
			}
			if srv.authHits != 1 {
				t.Errorf("refresh endpoint hit %d times, want 1", srv.authHits)
			}
			if len(srv.seenAuth) != 2 || srv.seenAuth[1] != "Bearer access-new" {
				t.Errorf("request auth headers = %v, want retry to carry access-new", srv.seenAuth)
			}
		})
	}
}

func TestTryRefreshToken_APITokenSession(t *testing.T) {
	c := New("https://sd.example.com", WithTokens("sd_api_token", ""))
	err := c.TryRefreshToken(context.Background())
	if err == nil {
		t.Fatal("expected error for API-token session")
	}
	if !strings.Contains(err.Error(), "https://sd.example.com/settings/api-tokens") {
		t.Fatalf("error should point at token settings page, got: %v", err)
	}
}

// A real permission denial is not an expired token: no refresh, and the body
// must reach the caller verbatim so the reason still renders.
func TestPlain403DoesNotRefresh(t *testing.T) {
	const denied = `{"code":403,"id":"forbidden","kind":"auth","reason":"insufficient permissions"}`
	srv := &refreshServer{
		t:            t,
		wantToken:    "nobody",
		rejectStatus: http.StatusForbidden,
		rejectBody:   denied,
	}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	c := New(ts.URL, WithTokens("access-old", "refresh-old"))

	_, httpResp, err := c.API().ApiV1ConfigGet(context.Background()).Execute()
	if err == nil {
		t.Fatal("expected 403 error")
	}
	if httpResp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", httpResp.StatusCode)
	}
	if srv.authHits != 0 {
		t.Errorf("refresh attempted for a plain 403 (%d hits)", srv.authHits)
	}
	if srv.configHits != 1 {
		t.Errorf("request sent %d times, want 1 (no retry)", srv.configHits)
	}
	if got := extractAPIReason(err); got != "insufficient permissions" {
		t.Errorf("reason = %q, want %q — body was not preserved", got, "insufficient permissions")
	}
	if b, ok := err.(bodyer); !ok || string(b.Body()) != denied {
		t.Errorf("error body = %q, want verbatim %q", b.Body(), denied)
	}
}
