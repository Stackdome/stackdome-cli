package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	"github.com/Stackdome/stackdome-cli/internal/output"
)

func TestValidatePrintsJSONSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stackfile.yaml")
	if err := os.WriteFile(path, []byte("name: demo\nresources:\n  web:\n    image: nginx:alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	ctx := cmdutil.NewCommandContext(&config.Config{}, output.FormatJSON, slog.LevelError)
	ctx.Formatter.Writer = &stdout
	cmd := newValidateCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--file", path})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	var result struct {
		Valid bool   `json:"valid"`
		File  string `json:"file"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("validate output is not JSON: %v\n%s", err, stdout.String())
	}
	if !result.Valid || result.File != path {
		t.Fatalf("result = %+v, want valid file %q", result, path)
	}
}

func TestValidateReturnsErrorForInvalidStackfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stackfile.yaml")
	if err := os.WriteFile(path, []byte("name: demo\nresources: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := cmdutil.NewCommandContext(&config.Config{}, output.FormatJSON, slog.LevelError)
	ctx.Formatter.Writer = &bytes.Buffer{}
	cmd := newValidateCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--file", path})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected invalid Stackfile to return an error")
	}
}

func TestValidateRejectsStackfileThatCannotConvertForDeploy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stackfile.yaml")
	content := `name: demo
resources:
  web:
    build:
      repo: https://github.com/example/app.git
      branch: main
      git_secret: legacy-git-credentials
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := cmdutil.NewCommandContext(&config.Config{}, output.FormatJSON, slog.LevelError)
	ctx.Formatter.Writer = &bytes.Buffer{}
	cmd := newValidateCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--file", path})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "git_secret") {
		t.Fatalf("validate error = %v, want unsupported git_secret error", err)
	}
}
