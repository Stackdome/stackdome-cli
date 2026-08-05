package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestStreamReleaseEventsResumesAfterDrop: the server hands out events 1-3 then
// drops the connection mid-release. The client must reconnect from where it
// left off (?after_sequence=3) and deliver 4-5 — every event exactly once.
func TestStreamReleaseEventsResumesAfterDrop(t *testing.T) {
	var mu sync.Mutex
	var seenAfter []string
	attempt := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempt++
		n := attempt
		seenAfter = append(seenAfter, r.URL.Query().Get("after_sequence"))
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		if n == 1 {
			for seq := 1; seq <= 3; seq++ {
				fmt.Fprintf(w, "id: %d\ndata: {\"sequence\":%d,\"message\":\"step %d\"}\n\n", seq, seq, seq)
			}
			flusher.Flush()
			return // handler returns == connection drops, no "end" event
		}

		for seq := 4; seq <= 5; seq++ {
			fmt.Fprintf(w, "id: %d\ndata: {\"sequence\":%d,\"message\":\"step %d\"}\n\n", seq, seq, seq)
		}
		fmt.Fprint(w, "event: end\ndata: {}\n\n")
		flusher.Flush()
	}))
	defer ts.Close()

	c := New(ts.URL, WithTokens("access", ""), WithOrgAndProject("org-1", "proj-1"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, err := c.StreamReleaseEvents(ctx, "stack-1", "rel-1", 0)
	if err != nil {
		t.Fatalf("StreamReleaseEvents: %v", err)
	}

	var got []string
	for e := range events {
		if e.Event == "error" {
			t.Fatalf("stream error event: %s", e.Data)
		}
		got = append(got, e.Data)
	}

	want := []string{
		`{"sequence":1,"message":"step 1"}`,
		`{"sequence":2,"message":"step 2"}`,
		`{"sequence":3,"message":"step 3"}`,
		`{"sequence":4,"message":"step 4"}`,
		`{"sequence":5,"message":"step 5"}`,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d events, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d: got %q, want %q", i, got[i], want[i])
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seenAfter) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(seenAfter))
	}
	if seenAfter[0] != "0" {
		t.Errorf("first connect after_sequence = %q, want %q", seenAfter[0], "0")
	}
	if seenAfter[1] != "3" {
		t.Errorf("reconnect after_sequence = %q, want %q", seenAfter[1], "3")
	}
}
