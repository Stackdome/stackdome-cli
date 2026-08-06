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
	if !strings.Contains(s, agentsStanzaBegin) || !strings.Contains(s, "docs.stackdome.io/guides/ai-agents") {
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
