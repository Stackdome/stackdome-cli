package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

const (
	agentsStanzaBegin = "<!-- stackdome:begin -->"
	agentsStanzaEnd   = "<!-- stackdome:end -->"
)

const agentsStanza = agentsStanzaBegin + `
## Deploying with Stackdome

This repo deploys to a Stackdome instance via the ` + "`stackdome`" + ` CLI.

- Full agent guide: https://stackdome.mintlify.app/guides/ai-agents.md
- Install CLI: ` + "`curl -fsSL https://stackdome.com/cli | sh`" + `
- Auth: ` + "`stackdome login --url <instance-url> --token <api-token>`" + ` (create tokens at ` + "`<instance-url>/settings/api-tokens`" + `)
- Deploy: ` + "`stackdome validate && stackdome deploy`" + ` (stackfile.yaml defines the stack)
- Inspect: ` + "`stackdome status -o json`" + `, ` + "`stackdome logs [resource]`" + `
` + agentsStanzaEnd + "\n"

// writeAgentsStanza upserts the marked block in <dir>/AGENTS.md, creating the file if needed.
func writeAgentsStanza(dir string) error {
	path := filepath.Join(dir, "AGENTS.md")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	s := string(existing)

	before, rest, found := strings.Cut(s, agentsStanzaBegin)
	if found {
		_, after, _ := strings.Cut(rest, agentsStanzaEnd)
		s = before + agentsStanza + strings.TrimPrefix(after, "\n")
	} else {
		if s = strings.TrimRight(s, "\n"); s != "" {
			s += "\n\n"
		}
		s += agentsStanza
	}
	return os.WriteFile(path, []byte(s), 0644)
}

// TTY: ask. Non-TTY (CI, agents): write without asking — that audience is who the stanza serves.
func shouldWriteAgentsMD() bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return true
	}
	answer := promptInput("Add Stackdome instructions to AGENTS.md? [Y/n]: ")
	return answer == "" || strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
}
