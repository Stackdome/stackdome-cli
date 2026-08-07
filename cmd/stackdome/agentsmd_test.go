package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAgentsStanza_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := writeAgentsStanza(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, agentsStanzaBegin) || !strings.Contains(s, agentsStanza) {
		t.Fatalf("stanza missing expected content:\n%s", s)
	}
}

func TestWriteAgentsStanza_Idempotent(t *testing.T) {
	dir := t.TempDir()
	if err := writeAgentsStanza(dir); err != nil {
		t.Fatal(err)
	}
	if err := writeAgentsStanza(dir); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if strings.Count(string(b), agentsStanzaBegin) != 1 {
		t.Fatalf("stanza duplicated:\n%s", string(b))
	}

	// Re-run with content before and after an already-written stanza, to exercise
	// the path that preserves everything past the end marker.
	dir2 := t.TempDir()
	seed := "# Heading\n\n" + agentsStanza + "## Later section\n\nMore content.\n"
	if err := os.WriteFile(filepath.Join(dir2, "AGENTS.md"), []byte(seed), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeAgentsStanza(dir2); err != nil {
		t.Fatal(err)
	}
	b2, _ := os.ReadFile(filepath.Join(dir2, "AGENTS.md"))
	s2 := string(b2)
	if strings.Count(s2, agentsStanzaBegin) != 1 {
		t.Fatalf("stanza duplicated:\n%s", s2)
	}
	if !strings.Contains(s2, "# Heading") {
		t.Fatal("heading before stanza lost")
	}
	if !strings.Contains(s2, "## Later section") {
		t.Fatal("content after stanza lost")
	}
}

func TestWriteAgentsStanza_PreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	orig := "# My project\n\nExisting instructions.\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(orig), 0644)
	if err := writeAgentsStanza(dir); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(b), "Existing instructions.") {
		t.Fatal("existing content lost")
	}
}
