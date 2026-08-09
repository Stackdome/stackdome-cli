package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestVerbFirstCommandPaths(t *testing.T) {
	paths := [][]string{
		{"get", "stacks"},
		{"list", "stacks"},
		{"get", "stack", "demo"},
		{"describe", "stack", "demo"},
		{"get", "builds"},
		{"list", "builds"},
		{"get", "build", "build-1"},
		{"describe", "build", "build-1"},
		{"get", "releases"},
		{"list", "releases"},
		{"get", "release", "release-1"},
		{"describe", "release", "release-1"},
		{"get", "release-events", "release-1"},
		{"list", "release-events", "release-1"},
		{"get", "secrets"},
		{"list", "secrets"},
		{"get", "secret", "api-key"},
		{"describe", "secret", "api-key"},
		{"get", "volumes"},
		{"list", "volumes"},
		{"get", "postgres-addons"},
		{"list", "postgres-addons"},
		{"get", "postgres-addon", "database"},
		{"describe", "postgres-addon", "database"},
		{"get", "postgres-backups", "database"},
		{"list", "postgres-backups", "database"},
		{"get", "postgres-credentials", "database", "app"},
		{"get", "tokens"},
		{"list", "tokens"},
		{"get", "token-scopes"},
		{"list", "token-scopes"},
		{"get", "config"},
		{"get", "stackfile-schema"},
		{"create", "release"},
		{"create", "secret", "api-key"},
		{"update", "secret", "api-key"},
		{"delete", "secret", "api-key"},
		{"create", "volume", "data"},
		{"delete", "volume", "data"},
		{"create", "postgres-addon", "database"},
		{"delete", "postgres-addon", "database"},
		{"create", "token", "ci"},
		{"delete", "token", "token-1"},
		{"delete", "stack", "demo"},
		{"cancel", "release", "release-1"},
		{"rollback", "release", "release-1"},
		{"backup", "postgres-addon", "database"},
		{"use", "stack", "demo"},
		{"use", "context", "https://api.stackdome.example"},
		{"export", "stackfile", "demo"},
	}

	root := newRootCmd()
	for _, path := range paths {
		path := path
		t.Run(strings.Join(path, "_"), func(t *testing.T) {
			cmd, _, err := root.Find(path)
			if err != nil {
				t.Fatalf("Find(%q): %v", path, err)
			}
			if cmd == root || !cmd.Runnable() {
				t.Fatalf("Find(%q) resolved to non-runnable %q", path, cmd.CommandPath())
			}
		})
	}
}

func TestWrongNounFormsReturnCorrectiveUsage(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"list", "build"}, want: "stackdome list builds"},
		{args: []string{"describe", "builds"}, want: "stackdome describe build <build-id>"},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		code := runWithWriters(tt.args, &stdout, &stderr)
		if code != 4 {
			t.Errorf("%v exit code = %d, want 4; stderr: %s", tt.args, code, stderr.String())
		}
		if !strings.Contains(stderr.String(), tt.want) {
			t.Errorf("%v stderr omitted %q:\n%s", tt.args, tt.want, stderr.String())
		}
	}
}

func TestExecutableCommandsHaveAgentReadableHelp(t *testing.T) {
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		if cmd.Runnable() {
			if strings.TrimSpace(cmd.Use) == "" || strings.TrimSpace(cmd.Short) == "" ||
				strings.TrimSpace(cmd.Long) == "" || strings.TrimSpace(cmd.Example) == "" {
				t.Errorf("%s has incomplete agent help (Use=%t Short=%t Long=%t Example=%t)",
					cmd.CommandPath(), strings.TrimSpace(cmd.Use) != "", strings.TrimSpace(cmd.Short) != "",
					strings.TrimSpace(cmd.Long) != "", strings.TrimSpace(cmd.Example) != "")
			}
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(newRootCmd())
}

func TestRootCommandsAreGroupedByPurpose(t *testing.T) {
	root := newRootCmd()
	wantGroups := map[string]bool{
		"read": false, "change": false, "deploy": false,
		"observe": false, "auth": false, "tooling": false,
	}
	for _, group := range root.Groups() {
		if _, ok := wantGroups[group.ID]; ok {
			wantGroups[group.ID] = true
		}
	}
	for group, found := range wantGroups {
		if !found {
			t.Errorf("root help group %q is not registered", group)
		}
	}
	for _, cmd := range root.Commands() {
		if cmd.Name() == "help" {
			continue
		}
		if cmd.GroupID == "" {
			t.Errorf("root command %q has no help group", cmd.Name())
		}
	}
}

func TestCtxIsRegisteredInAuthenticationGroup(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"ctx"})
	if err != nil || cmd == root || !cmd.Runnable() {
		t.Fatalf("ctx did not resolve to a runnable command: cmd=%v err=%v", cmd, err)
	}
	if cmd.GroupID != "auth" {
		t.Fatalf("ctx group = %q, want auth", cmd.GroupID)
	}
	text := cmd.Use + "\n" + cmd.Short + "\n" + cmd.Long + "\n" + cmd.Example
	for _, want := range []string{"ctx", "server", "stack", "stackdome ctx -o json"} {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(want)) {
			t.Fatalf("ctx help omitted %q:\n%s", want, text)
		}
	}
}

func TestSafetyCriticalHelpDocumentsSideEffects(t *testing.T) {
	root := newRootCmd()
	tests := []struct {
		path []string
		want []string
	}{
		{path: []string{"apply"}, want: []string{"does not create a release", "create release", "deploy"}},
		{path: []string{"create", "release"}, want: []string{"does not apply", "--wait", "--timeout"}},
		{path: []string{"deploy"}, want: []string{"saves the stack definition", "creates a release", "apply"}},
		{path: []string{"create", "token"}, want: []string{"sensitive", "once"}},
		{path: []string{"get", "postgres-credentials"}, want: []string{"sensitive", "secret"}},
		{path: []string{"delete", "stack"}, want: []string{"Permanently", "--yes"}},
		{path: []string{"list", "builds"}, want: []string{"equivalent", "stackdome get builds"}},
		{path: []string{"describe", "build"}, want: []string{"equivalent", "stackdome get build"}},
	}
	for _, tt := range tests {
		cmd, _, err := root.Find(tt.path)
		if err != nil {
			t.Fatalf("find %v: %v", tt.path, err)
		}
		text := cmd.Long + "\n" + cmd.Example
		for _, want := range tt.want {
			if !strings.Contains(strings.ToLower(text), strings.ToLower(want)) {
				t.Errorf("%s help omitted %q:\n%s", cmd.CommandPath(), want, text)
			}
		}
	}
}

func TestLifecycleCommandsRejectUnexpectedPositionalArguments(t *testing.T) {
	for _, args := range [][]string{
		{"apply", "unexpected"},
		{"deploy", "unexpected"},
		{"create", "release", "unexpected"},
		{"ctx", "unexpected"},
	} {
		var stdout, stderr bytes.Buffer
		if code := runWithWriters(args, &stdout, &stderr); code != 4 {
			t.Errorf("%v exit code = %d, want usage exit 4; stderr: %s", args, code, stderr.String())
		}
	}
}

func TestLogsBuildResolvesToBuildLogCommand(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"logs", "build", "build-1"})
	if err != nil {
		t.Fatalf("find logs build: %v", err)
	}
	if got := cmd.CommandPath(); got != "stackdome logs build" {
		t.Fatalf("command path = %q, want stackdome logs build", got)
	}
}

func TestLegacyResourceFirstPathsAreRemoved(t *testing.T) {
	legacyRoots := []string{
		"stack", "build", "release", "secret", "volume", "addon", "postgres",
		"token", "config", "stackfile", "destroy",
	}
	root := newRootCmd()
	for _, name := range legacyRoots {
		cmd, _, err := root.Find([]string{name})
		if err == nil && cmd != root && cmd.Name() == name {
			t.Errorf("legacy root %q still resolves to %s", name, cmd.CommandPath())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := runWithWriters([]string{"stack", "list"}, &stdout, &stderr); code != 4 {
		t.Errorf("removed command exit code = %d, want usage exit 4; stderr: %s", code, stderr.String())
	}
}
