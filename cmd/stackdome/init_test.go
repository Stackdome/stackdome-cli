package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"github.com/Stackdome/stackdome-cli/internal/stackfile"
)

const nestedComposeFixture = `services:
  web:
    image: nginx:alpine
    build:
      context: .
      dockerfile: Containerfile
      target: production
      args:
        MODE: release
      cache_from:
        - type=local
    depends_on:
      db:
        condition: service_healthy
        restart: true
        required: false
    volumes:
      - data:/srv/data:ro
      - cache:/srv/cache
    ports:
      - "8080:80"
      - "8000-8002:9000-9002"
      - not-a-port
  db:
    image: postgres:16
volumes:
  data:
    driver: local
    external: true
    driver_opts:
      type: none
    labels:
      tier: app
    name: actual-data
  cache: {}
`

const commandPortComposeFixture = `services:
  web:
    image: nginx:alpine
    command: /bin/sh -c 'echo "hello world"'
    ports:
      - "8081:81"
      - "127.0.0.1:8080:80"
      - "5353:5353/udp"
`

func TestInitWithoutComposeCreatesMinimalValidStackfileJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout bytes.Buffer
	ctx := cmdutil.NewCommandContext(&config.Config{}, output.FormatJSON, slog.LevelError)
	ctx.Formatter.Writer = &stdout
	cmd := newInitCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--name", "demo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	var result struct {
		Path      string   `json:"path"`
		Source    string   `json:"source"`
		Resources []string `json:"resources"`
		Volumes   []string `json:"volumes"`
		Warnings  []string `json:"warnings"`
		Valid     bool     `json:"valid"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("init output is not JSON: %v\n%s", err, stdout.String())
	}
	if result.Path != "stackfile.yaml" || result.Source != "template" || !result.Valid {
		t.Fatalf("unexpected init result: %+v", result)
	}
	if len(result.Resources) != 1 || result.Resources[0] != "web" {
		t.Fatalf("resources = %v, want [web]", result.Resources)
	}
	if len(result.Volumes) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("volumes/warnings = %v/%v, want empty", result.Volumes, result.Warnings)
	}

	content, err := os.ReadFile("stackfile.yaml")
	if err != nil {
		t.Fatal(err)
	}
	sf, err := stackfile.Load("stackfile.yaml")
	if err != nil {
		t.Fatalf("generated stackfile: %v\n%s", err, content)
	}
	web := sf.Resources["web"]
	if len(sf.Resources) != 1 || web.Image != "nginx:alpine" || len(web.Ports) != 1 || web.Ports[0].Port != 80 || !web.Ports[0].Public || web.Ports[0].Protocol != "" {
		t.Fatalf("generated resources = %+v, want one HTTP-defaultable public nginx:alpine port 80", sf.Resources)
	}
}

func TestInitComposeEnvFileReportsWarningWithoutEmbeddingIt(t *testing.T) {
	dir := t.TempDir()
	compose := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(compose, []byte("services:\n  web:\n    image: nginx:alpine\n    env_file:\n      - .env.base\n      - path: .env.override\n        required: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stdout bytes.Buffer
	ctx := cmdutil.NewCommandContext(&config.Config{}, output.FormatJSON, slog.LevelError)
	ctx.Formatter.Writer = &stdout
	cmd := newInitCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--name", "demo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	var result struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("init output is not JSON: %v\n%s", err, stdout.String())
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "env_file") || !strings.Contains(result.Warnings[0], ".env.base") || !strings.Contains(result.Warnings[0], ".env.override") {
		t.Fatalf("warnings = %v, want one warning naming every env_file", result.Warnings)
	}
	content, err := os.ReadFile(filepath.Join(dir, "stackfile.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "env_file") {
		t.Fatalf("generated stackfile must not embed unsupported env_file:\n%s", content)
	}
}

func TestInitWarnsForUnsupportedWindowsBindMount(t *testing.T) {
	dir := t.TempDir()
	compose := filepath.Join(dir, "compose.yaml")
	content := "services:\n  web:\n    image: nginx:alpine\n    volumes:\n      - 'C:\\data:/data'\n"
	if err := os.WriteFile(compose, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stdout bytes.Buffer
	ctx := cmdutil.NewCommandContext(&config.Config{}, output.FormatJSON, slog.LevelError)
	ctx.Formatter.Writer = &stdout
	cmd := newInitCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--name", "demo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	var result struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("init output is not JSON: %v\n%s", err, stdout.String())
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "bind mount") || !strings.Contains(result.Warnings[0], `C:\data`) {
		t.Fatalf("warnings = %v, want unsupported Windows bind mount warning", result.Warnings)
	}
}

func TestInitComposeWarningsAreDeterministicAndActionable(t *testing.T) {
	dir := t.TempDir()
	compose := filepath.Join(dir, "compose.yaml")
	content := `services:
  worker:
    image: busybox:latest
    environment:
      - WORKER_TOKEN
  web:
    image: nginx:alpine
    healthcheck:
      test: [CMD, curl, -f, http://localhost]
    deploy:
      replicas: 2
    networks: [frontend]
    profiles: [production]
    environment:
      API_TOKEN:
networks:
  frontend: {}
secrets:
  api-token:
    external: true
`
	if err := os.WriteFile(compose, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stdout bytes.Buffer
	ctx := cmdutil.NewCommandContext(&config.Config{}, output.FormatJSON, slog.LevelError)
	ctx.Formatter.Writer = &stdout
	cmd := newInitCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--name", "demo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	var result struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("init output is not JSON: %v\n%s", err, stdout.String())
	}
	want := []string{
		`compose used unsupported top-level keys "networks", "secrets" — review and recreate this behavior manually in stackfile.yaml`,
		`resource "web" used unsupported compose keys "deploy", "healthcheck", "networks", "profiles" — review and recreate this behavior manually in stackfile.yaml`,
		`resource "web" has unresolved environment variables "API_TOKEN" — set explicit non-sensitive values in env or connect a Stackdome secret; no values were imported`,
		`resource "worker" has unresolved environment variables "WORKER_TOKEN" — set explicit non-sensitive values in env or connect a Stackdome secret; no values were imported`,
	}
	if len(result.Warnings) != len(want) {
		t.Fatalf("warnings = %#v, want %#v", result.Warnings, want)
	}
	for i := range want {
		if result.Warnings[i] != want[i] {
			t.Errorf("warning %d = %q, want %q", i, result.Warnings[i], want[i])
		}
	}

	generated, err := os.ReadFile("stackfile.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(generated), "<nil>") || strings.Contains(string(generated), "API_TOKEN") || strings.Contains(string(generated), "WORKER_TOKEN") {
		t.Fatalf("generated Stackfile invented unresolved environment values:\n%s", generated)
	}
}

func TestInitReportsNestedComposeWarningsInDeterministicJSONOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(nestedComposeFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stdout bytes.Buffer
	ctx := cmdutil.NewCommandContext(&config.Config{}, output.FormatJSON, slog.LevelError)
	ctx.Formatter.Writer = &stdout
	cmd := newInitCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--name", "demo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	var result struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("init output is not JSON: %v\n%s", err, stdout.String())
	}
	want := []string{
		`compose volume "data" used unsupported options "driver", "driver_opts", "external", "labels", "name" — only the volume name was preserved with a default size; recreate storage settings manually`,
		`resource "web" has a local build (no git repo) — set build.repo to a git URL`,
		`resource "web" used unsupported build options "args", "cache_from", "target" — only build.context and build.dockerfile were preserved; recreate build settings manually`,
		`resource "web" used unsupported depends_on options "db.condition", "db.required", "db.restart" — dependency names were preserved; recreate ordering and health requirements manually`,
		`resource "web" used unsupported volume mount entries or options "data:/srv/data:ro" — only supported named-volume source and target pairs were preserved; recreate mount behavior manually`,
		`resource "web" used port mappings "8080:80" whose host IP or published port cannot be represented exactly — container ports were preserved; constrained host-IP bindings remain private, and published host ports require Stackdome routing`,
		`resource "web" used unsupported port entries "8000-8002:9000-9002", "not-a-port" — these ports were omitted; replace ranges, invalid syntax, or unsupported protocols with explicit TCP single-port mappings`,
	}
	if !reflect.DeepEqual(result.Warnings, want) {
		t.Fatalf("warnings = %#v, want %#v", result.Warnings, want)
	}
}

func TestInitReportsNestedComposeWarningsInDefaultTablePath(t *testing.T) {
	if os.Getenv("STACKDOME_TEST_INIT_NESTED_WARNINGS_HELPER") == "1" {
		os.Exit(runWithWriters([]string{"init", "--name", "demo"}, os.Stdout, os.Stderr))
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(nestedComposeFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestInitReportsNestedComposeWarningsInDefaultTablePath$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"STACKDOME_TEST_INIT_NESTED_WARNINGS_HELPER=1",
		"STACKDOME_CONFIG="+filepath.Join(dir, "config.yaml"),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("table init: %v\nstderr:\n%s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("table init stdout = %q, want empty", stdout.String())
	}
	orderedFragments := []string{
		`compose volume "data" used unsupported options`,
		`resource "web" has a local build`,
		`resource "web" used unsupported build options`,
		`resource "web" used unsupported depends_on options`,
		`resource "web" used unsupported volume mount entries or options`,
		`resource "web" used port mappings`,
		`resource "web" used unsupported port entries`,
	}
	last := -1
	for _, fragment := range orderedFragments {
		index := strings.Index(stderr.String(), fragment)
		if index < 0 {
			t.Fatalf("stderr omitted %q:\n%s", fragment, stderr.String())
		}
		if index <= last {
			t.Fatalf("stderr warning order is not deterministic:\n%s", stderr.String())
		}
		last = index
	}
}

func TestInitReportsCommandAndPortSemanticsWarningsInDeterministicJSONOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(commandPortComposeFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stdout bytes.Buffer
	ctx := cmdutil.NewCommandContext(&config.Config{}, output.FormatJSON, slog.LevelError)
	ctx.Formatter.Writer = &stdout
	cmd := newInitCmd()
	cmd.SetContext(context.Background())
	cmdutil.SetContext(cmd, ctx)
	cmd.SetArgs([]string{"--name", "demo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	var result struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("init output is not JSON: %v\n%s", err, stdout.String())
	}
	want := []string{
		`resource "web" used unsupported or ambiguous Compose command forms for "command" — exact argument, shell, or image-default semantics cannot be preserved; the values were omitted, so use explicit non-empty YAML string lists`,
		`resource "web" used port mappings "127.0.0.1:8080:80", "8081:81" whose host IP or published port cannot be represented exactly — container ports were preserved; constrained host-IP bindings remain private, and published host ports require Stackdome routing`,
		`resource "web" used unsupported port entries "5353:5353/udp" — these ports were omitted; replace ranges, invalid syntax, or unsupported protocols with explicit TCP single-port mappings`,
	}
	if !reflect.DeepEqual(result.Warnings, want) {
		t.Fatalf("warnings = %#v, want %#v", result.Warnings, want)
	}
}

func TestInitReportsCommandAndPortSemanticsWarningsInDefaultTablePath(t *testing.T) {
	if os.Getenv("STACKDOME_TEST_INIT_COMMAND_PORT_WARNINGS_HELPER") == "1" {
		os.Exit(runWithWriters([]string{"init", "--name", "demo"}, os.Stdout, os.Stderr))
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(commandPortComposeFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestInitReportsCommandAndPortSemanticsWarningsInDefaultTablePath$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"STACKDOME_TEST_INIT_COMMAND_PORT_WARNINGS_HELPER=1",
		"STACKDOME_CONFIG="+filepath.Join(dir, "config.yaml"),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("table init: %v\nstderr:\n%s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("table init stdout = %q, want empty", stdout.String())
	}
	commandIndex := strings.Index(stderr.String(), `resource "web" used unsupported or ambiguous Compose command forms for "command"`)
	portIndex := strings.Index(stderr.String(), `resource "web" used port mappings "127.0.0.1:8080:80", "8081:81"`)
	unsupportedIndex := strings.Index(stderr.String(), `resource "web" used unsupported port entries "5353:5353/udp"`)
	if commandIndex < 0 || portIndex < 0 || unsupportedIndex < 0 {
		t.Fatalf("stderr omitted command or port warning:\n%s", stderr.String())
	}
	if commandIndex >= portIndex || portIndex >= unsupportedIndex {
		t.Fatalf("stderr warning order is not deterministic:\n%s", stderr.String())
	}
}
