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

- Full agent guide: https://docs.stackdome.io/guides/ai-agents.md
- Install CLI: ` + "`curl -fsSL https://get.stackdome.com/cli | sh`" + `
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

	if i := strings.Index(s, agentsStanzaBegin); i >= 0 {
		j := strings.Index(s, agentsStanzaEnd)
		if j < 0 {
			j = len(s)
		} else {
			j += len(agentsStanzaEnd)
			if j < len(s) && s[j] == '\n' {
				j++
			}
		}
		s = s[:i] + agentsStanza + s[j:]
	} else {
		if s != "" && !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		if s != "" {
			s += "\n"
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
