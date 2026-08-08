package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Removing the direct request's configured transport or bearer header would
// prevent an authenticated API escape hatch from reaching the selected server.
func TestAPIRequestUsesConfiguredServerAndBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.RequestURI(), "/api/v1/users/current?verbose=true"; got != want {
			t.Errorf("request URI = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer sdm_test"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"user-1"}`))
	}))
	defer server.Close()

	c := New(server.URL, WithTokens("sdm_test", ""))
	response, err := c.APIRequest(context.Background(), http.MethodGet, "/api/v1/users/current?verbose=true", nil, nil)
	if err != nil {
		t.Fatalf("APIRequest: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got, want := string(response.Body), `{"id":"user-1"}`; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

// The direct API request must use the normal refresh-aware transport rather
// than treating a 401 as a terminal response.
func TestAPIRequestRefreshesAfter401(t *testing.T) {
	protectedHits := 0
	refreshHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/protected":
			protectedHits++
			if got := r.Header.Get("Authorization"); protectedHits == 1 && got != "Bearer access-old" {
				t.Errorf("initial Authorization = %q, want old token", got)
			}
			if protectedHits == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if got, want := r.Header.Get("Authorization"), "Bearer access-new"; got != want {
				t.Errorf("retried Authorization = %q, want %q", got, want)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/v1/auth/refresh":
			refreshHits++
			var body struct {
				RefreshToken string `json:"refreshToken"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode refresh body: %v", err)
			}
			if got, want := body.RefreshToken, "refresh-old"; got != want {
				t.Errorf("refresh token = %q, want %q", got, want)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"access-new","refreshToken":"refresh-new"}`))
		default:
			t.Errorf("unexpected request: %s", r.URL)
		}
	}))
	defer server.Close()

	c := New(server.URL, WithTokens("access-old", "refresh-old"))
	response, err := c.APIRequest(context.Background(), http.MethodGet, "/api/v1/protected", nil, nil)
	if err != nil {
		t.Fatalf("APIRequest: %v", err)
	}
	if response.StatusCode != http.StatusOK || string(response.Body) != `{"ok":true}` {
		t.Errorf("response = %#v, want refreshed 200 JSON response", response)
	}
	if protectedHits != 2 || refreshHits != 1 {
		t.Errorf("protected hits = %d, refresh hits = %d; want 2 and 1", protectedHits, refreshHits)
	}
}

// A refresh request carries the long-lived refresh token. A 307/308 must not
// replay that POST to a foreign origin after an API request receives a 401.
func TestAPIRequestRejectsCrossOriginRefreshRedirect(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			foreignRequests := 0
			foreignBody := ""
			foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				foreignRequests++
				body, _ := io.ReadAll(r.Body)
				foreignBody = string(body)
			}))
			defer foreign.Close()

			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/protected":
					w.WriteHeader(http.StatusUnauthorized)
				case "/api/v1/auth/refresh":
					w.Header().Set("Location", foreign.URL+"/refresh")
					w.WriteHeader(status)
				default:
					t.Errorf("unexpected request: %s", r.URL)
				}
			}))
			defer origin.Close()

			c := New(origin.URL, WithTokens("access-old", "refresh-secret"))
			response, err := c.APIRequest(context.Background(), http.MethodGet, "/api/v1/protected", nil, nil)
			if err != nil {
				t.Fatalf("APIRequest: %v", err)
			}
			if response.StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want original 401", response.StatusCode)
			}
			if foreignRequests != 0 || foreignBody != "" {
				t.Errorf("foreign requests = %d, body = %q; want no refresh replay", foreignRequests, foreignBody)
			}
		})
	}
}

// A custom redirect policy must retain a bounded same-origin chain rather
// than allowing an attacker-controlled loop until the request timeout.
func TestAPIRequestLimitsSameOriginRedirects(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits <= 11 {
			http.Redirect(w, r, "/api/v1/redirect", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`{"unexpected":"success"}`))
	}))
	defer server.Close()

	c := New(server.URL, WithTokens("sdm_test", ""))
	_, err := c.APIRequest(context.Background(), http.MethodGet, "/api/v1/redirect", nil, nil)
	if err == nil {
		t.Fatal("APIRequest error = nil, want redirect limit error")
	}
	if hits > 10 {
		t.Errorf("redirect hits = %d, want at most 10", hits)
	}
}

// Prefix checks alone are insufficient: URL dot segments can resolve an API
// path to a different endpoint before the request is sent.
func TestAPIRequestRejectsPathsEscapingAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request outside API prefix: %s", r.URL)
	}))
	defer server.Close()

	c := New(server.URL, WithTokens("sdm_test", ""))
	for _, path := range []string{
		"/api/../settings",
		"/api/%2e%2e/settings",
		"/api/%2E%2E%2Fsettings",
		"/api/%252e%252e/settings",
		"/api/%252E%252E%252Fsettings",
	} {
		t.Run(path, func(t *testing.T) {
			_, err := c.APIRequest(context.Background(), http.MethodGet, path, nil, nil)
			if err == nil {
				t.Fatal("APIRequest error = nil, want API-prefix escape rejection")
			}
		})
	}
}

// The contract requires PATH to begin with /api/, not merely to normalize to
// the /api route itself.
func TestAPIRequestRejectsBareAPIPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to bare API path: %s", r.URL)
	}))
	defer server.Close()

	c := New(server.URL, WithTokens("sdm_test", ""))
	if _, err := c.APIRequest(context.Background(), http.MethodGet, "/api", nil, nil); err == nil {
		t.Fatal("APIRequest error = nil, want bare /api rejection")
	}
}

// Rejecting every redirect would break normal same-server routing, while
// allowing a different origin could forward an API token to an attacker.
func TestAPIRequestAllowsSameOriginRedirect(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/api/v1/redirect" {
			http.Redirect(w, r, "/api/v1/final", http.StatusFound)
			return
		}
		if got, want := r.URL.Path, "/api/v1/final"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer sdm_test"; got != want {
			t.Errorf("redirect authorization = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	c := New(server.URL, WithTokens("sdm_test", ""))
	response, err := c.APIRequest(context.Background(), http.MethodGet, "/api/v1/redirect", nil, nil)
	if err != nil {
		t.Fatalf("APIRequest: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if requests != 2 {
		t.Errorf("requests = %d, want 2", requests)
	}
}

// Accepting an absolute or protocol-relative path would let a caller choose
// the credential destination instead of the configured Stackdome server.
func TestAPIRequestRejectsAbsoluteAndProtocolRelativePaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to configured server: %s", r.URL)
	}))
	defer server.Close()

	c := New(server.URL, WithTokens("sdm_test", ""))
	for _, path := range []string{"https://example.test/api/v1/users/current", "//example.test/api/v1/users/current"} {
		t.Run(path, func(t *testing.T) {
			_, err := c.APIRequest(context.Background(), http.MethodGet, path, nil, nil)
			if err == nil {
				t.Fatal("APIRequest error = nil, want unsafe target rejection")
			}
		})
	}
}

// A cross-origin redirect must fail before the redirected server sees the
// request, especially its Authorization header.
func TestAPIRequestRejectsCrossOriginRedirectBeforeSendingCredentials(t *testing.T) {
	var targetRequests int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetRequests++
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want no credential", got)
		}
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/api/v1/stolen", http.StatusFound)
	}))
	defer origin.Close()

	c := New(origin.URL, WithTokens("sdm_test", ""))
	_, err := c.APIRequest(context.Background(), http.MethodGet, "/api/v1/redirect", nil, nil)
	if err == nil {
		t.Fatal("APIRequest error = nil, want cross-origin redirect rejection")
	}
	if !strings.Contains(err.Error(), "origin") {
		t.Errorf("error = %q, want origin rejection", err)
	}
	if targetRequests != 0 {
		t.Errorf("cross-origin target requests = %d, want 0", targetRequests)
	}
}
