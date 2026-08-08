package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamingEndpointsPreserveRejectedAPITokenGuidance(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"reason":"token expired"}`))
	}))
	defer ts.Close()

	const rejectedToken = "sdm_rejected_stream_token"
	c := New(ts.URL, WithTokens(rejectedToken, ""), WithOrgAndProject("org-1", "default"))
	tests := []struct {
		name string
		call func() error
	}{
		{name: "runtime logs", call: func() error {
			_, err := c.StreamLogs(context.Background(), "stack-1", "web", LogOptions{Tail: 10})
			return err
		}},
		{name: "release events", call: func() error {
			_, err := c.StreamReleaseEvents(context.Background(), "stack-1", "release-1", 0)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("stream succeeded, want rejected-token error")
			}
			message := err.Error()
			if !strings.Contains(message, ts.URL+"/settings/api-tokens") || !strings.Contains(message, "stackdome login --token") {
				t.Fatalf("error = %q, want replacement-token guidance", message)
			}
			if strings.Contains(message, rejectedToken) {
				t.Fatalf("error leaked rejected token: %q", message)
			}
		})
	}
}

// A stream client intentionally omits the normal whole-request timeout, but it
// must retain the configured redirect policy: a redirect may otherwise replay
// the bearer token to a different origin.
func TestStreamingEndpointsRejectCrossOriginRedirectBeforeForeignRequest(t *testing.T) {
	foreignRequests := 0
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignRequests++
		t.Errorf("foreign server received %s with Authorization %q", r.URL, r.Header.Get("Authorization"))
	}))
	defer foreign.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/stolen", http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	c := New(origin.URL, WithTokens("stream-secret", ""), WithOrgAndProject("org-1", "proj-1"))
	for _, tt := range []struct {
		name string
		call func() error
	}{
		{name: "runtime logs", call: func() error {
			_, err := c.StreamLogs(context.Background(), "stack-1", "web", LogOptions{Follow: true})
			return err
		}},
		{name: "build logs", call: func() error {
			_, err := c.StreamBuildLogs(context.Background(), "stack-1", "build-1", LogOptions{Follow: true})
			return err
		}},
		{name: "release events", call: func() error {
			_, err := c.StreamReleaseEvents(context.Background(), "stack-1", "release-1", 0)
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("stream succeeded after a foreign redirect")
			}
			if foreignRequests != 0 {
				t.Fatalf("foreign requests = %d, want 0", foreignRequests)
			}
		})
	}
}

// Redirects used for routing within the configured service remain valid for
// every no-timeout stream endpoint, and keep the bearer token on that origin.
func TestStreamingEndpointsAllowSameOriginRedirect(t *testing.T) {
	const token = "stream-secret"
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/redirected" {
			http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
			return
		}
		if got, want := r.Header.Get("Authorization"), "Bearer "+token; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: end\ndata: {}\n\n")
	}))
	defer origin.Close()

	c := New(origin.URL, WithTokens(token, ""), WithOrgAndProject("org-1", "proj-1"))
	for _, tt := range []struct {
		name string
		call func() error
	}{
		{name: "runtime logs", call: func() error {
			body, err := c.StreamLogs(context.Background(), "stack-1", "web", LogOptions{Follow: true})
			if err != nil {
				return err
			}
			defer body.Close()
			return ParseSSEStream(body, func(SSEEvent) error { return nil })
		}},
		{name: "build logs", call: func() error {
			body, err := c.StreamBuildLogs(context.Background(), "stack-1", "build-1", LogOptions{Follow: true})
			if err != nil {
				return err
			}
			defer body.Close()
			return ParseSSEStream(body, func(SSEEvent) error { return nil })
		}},
		{name: "release events", call: func() error {
			events, err := c.StreamReleaseEvents(context.Background(), "stack-1", "release-1", 0)
			if err != nil {
				return err
			}
			for range events {
			}
			return nil
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err != nil {
				t.Fatalf("same-origin redirected stream: %v", err)
			}
		})
	}
}

// Host suffix matching is not origin matching. This exercises a real redirect
// with a parent hostname and a subdomain target, while the test transport maps
// both symbolic hosts to local servers.
func TestStreamLogsRejectsParentToSubdomainRedirect(t *testing.T) {
	foreignRequests := 0
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignRequests++
		t.Errorf("subdomain received Authorization %q", r.Header.Get("Authorization"))
	}))
	defer foreign.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://logs.api.stackdome.test/stolen", http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	dialer := &net.Dialer{}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		switch address {
		case "api.stackdome.test:80":
			return dialer.DialContext(ctx, network, origin.Listener.Addr().String())
		case "logs.api.stackdome.test:80":
			return dialer.DialContext(ctx, network, foreign.Listener.Addr().String())
		default:
			return nil, fmt.Errorf("unexpected dial address %q", address)
		}
	}}
	c := New("http://api.stackdome.test", WithTokens("stream-secret", ""), WithOrgAndProject("org-1", "proj-1"))
	c.cfg.HTTPClient.Transport = &refreshTransport{base: transport, client: c}

	if _, err := c.StreamLogs(context.Background(), "stack-1", "web", LogOptions{Follow: true}); err == nil {
		t.Fatal("stream succeeded after parent-to-subdomain redirect")
	}
	if foreignRequests != 0 {
		t.Fatalf("subdomain requests = %d, want 0", foreignRequests)
	}
}

// Retaining the transport is essential for TLS options supplied by the client
// configuration; only the client timeout is deliberately omitted for streams.
func TestStreamLogsRetainsConfiguredTLSTransport(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: end\ndata: {}\n\n")
	}))
	defer server.Close()

	c := New(server.URL, WithInsecure(true), WithTokens("stream-secret", ""), WithOrgAndProject("org-1", "proj-1"))
	body, err := c.StreamLogs(context.Background(), "stack-1", "web", LogOptions{Follow: true})
	if err != nil {
		t.Fatalf("StreamLogs with configured TLS transport: %v", err)
	}
	defer body.Close()
	if err := ParseSSEStream(body, func(SSEEvent) error { return nil }); err != nil {
		t.Fatalf("ParseSSEStream: %v", err)
	}
}

// Copying the redirect policy must not bypass the refresh-aware transport.
func TestStreamLogsRefreshesAfterUnauthorized(t *testing.T) {
	streamRequests := 0
	refreshRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			refreshRequests++
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"token":"access-new","refreshToken":"refresh-new"}`)
		default:
			streamRequests++
			if got, want := r.Header.Get("Authorization"), "Bearer access-new"; streamRequests == 2 && got != want {
				t.Errorf("refreshed Authorization = %q, want %q", got, want)
			}
			if streamRequests == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: end\ndata: {}\n\n")
		}
	}))
	defer server.Close()

	c := New(server.URL, WithTokens("access-old", "refresh-old"), WithOrgAndProject("org-1", "proj-1"))
	body, err := c.StreamLogs(context.Background(), "stack-1", "web", LogOptions{Follow: true})
	if err != nil {
		t.Fatalf("StreamLogs after refresh: %v", err)
	}
	defer body.Close()
	if err := ParseSSEStream(body, func(SSEEvent) error { return nil }); err != nil {
		t.Fatalf("ParseSSEStream: %v", err)
	}
	if streamRequests != 2 || refreshRequests != 1 {
		t.Errorf("stream requests = %d, refresh requests = %d; want 2 and 1", streamRequests, refreshRequests)
	}
}
