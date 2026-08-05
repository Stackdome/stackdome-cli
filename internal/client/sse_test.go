package client

import (
	"strings"
	"testing"
)

func TestParseSSEStream_DataEvents(t *testing.T) {
	input := "data: [web]: hello world\n\ndata: [web]: second line\n\n"
	var events []SSEEvent
	err := ParseSSEStream(strings.NewReader(input), func(e SSEEvent) error {
		events = append(events, e)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Data != "[web]: hello world" {
		t.Errorf("expected '[web]: hello world', got %q", events[0].Data)
	}
	if events[0].Event != "" {
		t.Errorf("expected empty event type, got %q", events[0].Event)
	}
}

func TestParseSSEStream_ErrorEvent(t *testing.T) {
	input := "event: error\ndata: connection lost\n\n"
	var events []SSEEvent
	err := ParseSSEStream(strings.NewReader(input), func(e SSEEvent) error {
		events = append(events, e)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Event != "error" {
		t.Errorf("expected event type 'error', got %q", events[0].Event)
	}
	if events[0].Data != "connection lost" {
		t.Errorf("expected 'connection lost', got %q", events[0].Data)
	}
}

func TestParseSSEStream_EmptyData(t *testing.T) {
	input := "data: \n\n"
	var events []SSEEvent
	err := ParseSSEStream(strings.NewReader(input), func(e SSEEvent) error {
		events = append(events, e)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Data != "" {
		t.Errorf("expected empty data, got %q", events[0].Data)
	}
}

func TestParseSSEStream_EmptyInput(t *testing.T) {
	var events []SSEEvent
	err := ParseSSEStream(strings.NewReader(""), func(e SSEEvent) error {
		events = append(events, e)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestParseSSEStream_CallbackError(t *testing.T) {
	input := "data: line1\n\ndata: line2\n\n"
	count := 0
	err := ParseSSEStream(strings.NewReader(input), func(e SSEEvent) error {
		count++
		if count == 1 {
			return &testErr{"stop"}
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected error from callback")
	}
	if count != 1 {
		t.Errorf("expected callback called once, got %d", count)
	}
}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }

func TestSSEEventIsEnd(t *testing.T) {
	cases := []struct {
		event SSEEvent
		want  bool
	}{
		{SSEEvent{Event: "end", Data: "{}"}, true},
		{SSEEvent{Event: "end"}, true},
		{SSEEvent{Data: "{}"}, false}, // a log line that happens to be `{}` still prints
		{SSEEvent{Data: "[web]: hello"}, false},
		{SSEEvent{Data: ""}, false},
	}
	for _, c := range cases {
		if got := c.event.IsEnd(); got != c.want {
			t.Errorf("IsEnd(%+v) = %v, want %v", c.event, got, c.want)
		}
	}
}
