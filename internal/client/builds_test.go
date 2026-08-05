package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestStreamBuildLogs pins the build-log URL (path + query) and the SSE frames
// the caller sees.
func TestStreamBuildLogs(t *testing.T) {
	var gotURL string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: step 1\n\ndata: step 2\n\nevent: end\ndata: \n\n")
	}))
	defer ts.Close()

	c := New(ts.URL, WithTokens("access", ""), WithOrgAndProject("org-1", "proj-1"))

	stream, err := c.StreamBuildLogs(context.Background(), "stack-1", "build-1", LogOptions{Follow: true, Tail: 200})
	if err != nil {
		t.Fatalf("StreamBuildLogs: %v", err)
	}
	defer stream.Close()

	var lines []string
	var ended bool
	if err := ParseSSEStream(stream, func(e SSEEvent) error {
		if e.Event == "end" {
			ended = true
			return nil
		}
		lines = append(lines, e.Data)
		return nil
	}); err != nil {
		t.Fatalf("ParseSSEStream: %v", err)
	}

	want := "/api/v1/organizations/org-1/projects/proj-1/stacks/stack-1/builds/build-1/logs?tail=200&follow=true"
	if gotURL != want {
		t.Errorf("URL = %q, want %q", gotURL, want)
	}
	if len(lines) != 2 || lines[0] != "step 1" || lines[1] != "step 2" {
		t.Errorf("lines = %v, want [step 1 step 2]", lines)
	}
	if !ended {
		t.Error("missing end event")
	}
}

// A followed log stream must outlive the client's whole-request timeout — only
// the context ends it.
func TestStreamLogsFollowOutlivesClientTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(150 * time.Millisecond)
		fmt.Fprint(w, "data: second\n\nevent: end\ndata: \n\n")
	}))
	defer ts.Close()

	c := New(ts.URL, WithTokens("access", ""), WithOrgAndProject("org-1", "proj-1"))
	c.cfg.HTTPClient.Timeout = 50 * time.Millisecond

	stream, err := c.StreamLogs(context.Background(), "stack-1", "web", LogOptions{Follow: true, Tail: 10})
	if err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	defer stream.Close()

	var lines []string
	if err := ParseSSEStream(stream, func(e SSEEvent) error {
		if e.Event != "end" {
			lines = append(lines, e.Data)
		}
		return nil
	}); err != nil {
		t.Fatalf("ParseSSEStream: %v", err)
	}

	if len(lines) != 2 || lines[1] != "second" {
		t.Errorf("lines = %v, want [first second] — stream was cut by the client timeout", lines)
	}
}
