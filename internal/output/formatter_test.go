package output

import (
	"bytes"
	"strings"
	"testing"
)

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
