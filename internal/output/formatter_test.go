package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// PrintJSONLine is the stream-safe counterpart to PrintJSON: every emitted
// event occupies exactly one compact, independently decodable line.
func TestPrintJSONLineWritesOneCompactDocument(t *testing.T) {
	var buf bytes.Buffer
	f := &Formatter{Format: FormatJSON, Writer: &buf}

	if err := f.PrintJSONLine(map[string]any{"event": "log", "data": map[string]any{"message": "hello"}}); err != nil {
		t.Fatalf("PrintJSONLine: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "\n  ") {
		t.Fatalf("stream document is indented: %q", got)
	}
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("line count = %d, want 1: %q", strings.Count(got, "\n"), got)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &line); err != nil {
		t.Fatalf("stream line is not JSON: %v", err)
	}
	if line["event"] != "log" {
		t.Errorf("event = %#v, want log", line["event"])
	}
}

// The API types carry only `json:` tags, so YAML must go through JSON to keep
// the wire names instead of emitting lowercased Go field names.
func TestPrintYAMLUsesJSONTags(t *testing.T) {
	var buf bytes.Buffer
	f := &Formatter{Format: FormatYAML, Writer: &buf}

	v := struct {
		ServerURL string `json:"server_url"`
		Hidden    string `json:"-"`
	}{ServerURL: "https://example.test", Hidden: "secret"}

	if err := f.PrintYAML(v); err != nil {
		t.Fatalf("PrintYAML: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "server_url: https://example.test") {
		t.Errorf("want json tag name in output, got:\n%s", got)
	}
	if strings.Contains(got, "secret") {
		t.Errorf("json:\"-\" field leaked into YAML:\n%s", got)
	}
}
