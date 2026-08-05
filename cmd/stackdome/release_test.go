package main

import (
	"encoding/json"
	"testing"
)

func TestEventDataJSONStaysValid(t *testing.T) {
	for _, data := range []string{`{"state":"Released"}`, "plain text \"quoted\"", ""} {
		line := `{"event":"message","data":` + eventDataJSON(data) + `}`
		if !json.Valid([]byte(line)) {
			t.Errorf("invalid NDJSON line for payload %q: %s", data, line)
		}
	}
}

func TestRedactSecret(t *testing.T) {
	if got := redactSecret("sdm_abcdefghijklmnop"); got != "sdm_abcd..." {
		t.Errorf("long token: got %q", got)
	}
	if got := redactSecret("short"); got != "..." {
		t.Errorf("short token: got %q", got)
	}
	if got := redactSecret(""); got != "" {
		t.Errorf("empty token: got %q", got)
	}
}
