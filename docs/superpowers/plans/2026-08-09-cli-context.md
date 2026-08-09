# CLI Context Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `stackdome ctx` and make the selected stack explicit in table, JSON, and YAML stack listings.

**Architecture:** Keep context presentation in a focused `ctx.go` command while extracting the small authentication-label calculation shared with `whoami`. Centralize current-stack matching and stack-list decoration in `stack.go` so `ctx`, `get stacks`, and `list stacks` use identical ID/name compatibility rules.

**Tech Stack:** Go 1.25, Cobra, generated Stackdome OpenAPI models, `httptest`, standard `encoding/json`, and the existing output formatter.

## Global Constraints

- `ctx` is a root command in the Authentication and Context help group.
- `ctx` accepts no positional arguments and supports table, JSON, and YAML output.
- Never print credentials or token fragments.
- A selected stack is resolved by full ID or legacy stored name.
- An unresolved saved selection returns exit code 3.
- Stack list table output uses an explicit `CURRENT` column with `*` on exactly one matching row.
- Stack list JSON/YAML entries retain existing stack fields and add `current: true|false`.
- Environment-controlled contexts must not expose the file-backed selected stack.

---

### Task 1: Decorate Stack Lists With Current Selection

**Files:**
- Modify: `cmd/stackdome/stack.go`
- Modify: `cmd/stackdome/stack_test.go`

**Interfaces:**
- Consumes: `config.Config.CurrentStack`, `openapi.Stack`, `output.Formatter`.
- Produces: `currentStackMatches(openapi.Stack, string) bool`, `stackListItem`, and `decorateStackList([]openapi.Stack, string) []stackListItem` for Task 2.

- [ ] **Step 1: Write failing table-output tests**

Add a helper-backed test that executes both aliases against an `httptest.Server` and asserts the rendered table has an explicit `CURRENT` header and marks the selected ID. Repeat with a legacy name selection:

```go
func TestStackListTableMarksCurrentStackForBothAliases(t *testing.T) {
    for _, args := range [][]string{{"get", "stacks"}, {"list", "stacks"}} {
        t.Run(strings.Join(args, "_"), func(t *testing.T) {
            stdout := executeStackList(t, output.FormatTable, "stack-2", args...)
            if !strings.Contains(stdout, "CURRENT") {
                t.Fatalf("table omitted CURRENT header:\n%s", stdout)
            }
            if !regexp.MustCompile(`(?m)^\*\s+two\s+stack-2\s+Released`).MatchString(stdout) {
                t.Fatalf("selected stack was not marked:\n%s", stdout)
            }
        })
    }
}

func TestStackListTableRecognizesLegacySelectedStackName(t *testing.T) {
    stdout := executeStackList(t, output.FormatTable, "two", "get", "stacks")
    if !regexp.MustCompile(`(?m)^\*\s+two\s+stack-2\s+Released`).MatchString(stdout) {
        t.Fatalf("legacy name selection was not marked:\n%s", stdout)
    }
}
```

The helper server returns two stack objects with IDs `stack-1` and `stack-2`, names `one` and `two`, and released latest releases. It builds a root command context with the requested output format and `CurrentStack` value.

Add the concrete helpers and required `regexp` / YAML imports:

```go
func executeStackList(t *testing.T, format output.Format, current string, args ...string) string {
    t.Helper()
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        if r.URL.Path != "/api/v1/organizations/org-1/projects/default/stacks" {
            t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
            w.WriteHeader(http.StatusNotFound)
            return
        }
        _, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"one","spec":{},"latest_release":{"state":"Released"}},{"id":"stack-2","name":"two","spec":{},"latest_release":{"state":"Released"}}],"total":2}`))
    }))
    defer server.Close()

    configPath := filepath.Join(t.TempDir(), "config.json")
    body := fmt.Sprintf(`{"server_url":%q,"access_token":"sdm_test","organization_id":"org-1","project_name":"default","current_stack":%q}`, server.URL, current)
    if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
        t.Fatal(err)
    }
    t.Setenv("STACKDOME_CONFIG", configPath)

    var stdout, stderr bytes.Buffer
    argv := append(append([]string{}, args...), "-o", string(format))
    if code := runWithWriters(argv, &stdout, &stderr); code != 0 {
        t.Fatalf("%v exit=%d stderr=%s", argv, code, stderr.String())
    }
    return stdout.String()
}

func decodeStructuredTestOutput(t *testing.T, format output.Format, raw string, target any) {
    t.Helper()
    var err error
    if format == output.FormatYAML {
        err = yaml.Unmarshal([]byte(raw), target)
    } else {
        err = json.Unmarshal([]byte(raw), target)
    }
    if err != nil {
        t.Fatalf("decode %s output: %v\n%s", format, err, raw)
    }
}
```

- [ ] **Step 2: Run the table tests and verify RED**

Run:

```bash
go test ./cmd/stackdome -run 'TestStackListTable(Marks|Recognizes)' -count=1
```

Expected: FAIL because the header is currently blank and name-based selections are not marked.

- [ ] **Step 3: Write failing structured-output tests**

Add JSON and YAML tests for both aliases. Decode output into objects and assert the first row is `current:false`, the second is `current:true`, and existing fields such as `id`, `name`, and `latest_release` remain present:

```go
func TestStackListStructuredOutputMarksCurrentStack(t *testing.T) {
    for _, format := range []output.Format{output.FormatJSON, output.FormatYAML} {
        for _, args := range [][]string{{"get", "stacks"}, {"list", "stacks"}} {
            t.Run(string(format)+"_"+strings.Join(args, "_"), func(t *testing.T) {
                stdout := executeStackList(t, format, "stack-2", args...)
                var items []map[string]any
                decodeStructuredTestOutput(t, format, stdout, &items)
                if items[0]["current"] != false || items[1]["current"] != true {
                    t.Fatalf("current flags = %#v", items)
                }
                if items[1]["id"] != "stack-2" || items[1]["name"] != "two" || items[1]["latest_release"] == nil {
                    t.Fatalf("existing stack fields were not preserved: %#v", items[1])
                }
            })
        }
    }
}
```

- [ ] **Step 4: Run the structured tests and verify RED**

Run:

```bash
go test ./cmd/stackdome -run TestStackListStructuredOutputMarksCurrentStack -count=1
```

Expected: FAIL because raw OpenAPI stack objects do not contain `current`.

- [ ] **Step 5: Implement list decoration and the explicit column**

In `stack.go`, add:

```go
type stackListItem struct {
    openapi.Stack
    Current bool `json:"current"`
}

func currentStackMatches(stack openapi.Stack, current string) bool {
    return current != "" && (stack.GetId() == current || stack.Name == current)
}

func decorateStackList(stacks []openapi.Stack, current string) []stackListItem {
    items := make([]stackListItem, len(stacks))
    for i := range stacks {
        items[i] = stackListItem{Stack: stacks[i], Current: currentStackMatches(stacks[i], current)}
    }
    return items
}
```

Pass decorated items to `PrintStructured`. Change the table construction to:

```go
tbl := ctx.Formatter.NewTable("CURRENT", "NAME", "ID", "STATE")
```

Use `currentStackMatches(s, ctx.Config.CurrentStack)` for the marker.

- [ ] **Step 6: Run stack tests and verify GREEN**

Run:

```bash
go test ./cmd/stackdome -run 'TestStack(List|Use|Info)' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit the stack-list behavior**

```bash
git add cmd/stackdome/stack.go cmd/stackdome/stack_test.go
git commit -m "feat: mark the current stack in list output"
```

---

### Task 2: Add the `ctx` Context Summary Command

**Files:**
- Create: `cmd/stackdome/ctx.go`
- Create: `cmd/stackdome/ctx_test.go`
- Modify: `cmd/stackdome/whoami.go`
- Modify: `cmd/stackdome/root.go`
- Modify: `cmd/stackdome/command_tree.go`
- Modify: `cmd/stackdome/command_tree_test.go`

**Interfaces:**
- Consumes: `currentStackMatches` and `decorateStackList` matching semantics from Task 1, `cmdutil.RequireAuth`, `Client.GetCurrentUser`, and `Client.ListStacks`.
- Produces: `newCtxCmd() *cobra.Command`, `authDetails(*config.Config) (string, string)`, `ctxInfo`, and `ctxStack`.

- [ ] **Step 1: Write failing registration and help tests**

Extend command-tree tests:

```go
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
```

Add `{"ctx", "unexpected"}` to the invocation table in
`TestLifecycleCommandsRejectUnexpectedPositionalArguments`.

- [ ] **Step 2: Run registration tests and verify RED**

Run:

```bash
go test ./cmd/stackdome -run 'TestCtx|TestLifecycleCommandsRejectUnexpectedPositionalArguments' -count=1
```

Expected: FAIL because `ctx` is not registered.

- [ ] **Step 3: Write failing live-context output tests**

Create `ctx_test.go` with an `httptest.Server` handling current-user and stack-list endpoints. Execute the root command in JSON, YAML, and table modes. Assert the structured object and table labels:

```go
type ctxTestResult struct {
    User           string    `json:"user" yaml:"user"`
    Email          string    `json:"email" yaml:"email"`
    ServerURL      string    `json:"server_url" yaml:"server_url"`
    OrganizationID string    `json:"organization_id" yaml:"organization_id"`
    Project        string    `json:"project" yaml:"project"`
    CurrentStack   *ctxStack `json:"current_stack" yaml:"current_stack"`
    AuthMethod     string    `json:"auth_method" yaml:"auth_method"`
    TokenSource    string    `json:"token_source" yaml:"token_source"`
}

func executeCtx(t *testing.T, format, current string) (stdout, stderr string, code int) {
    t.Helper()
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        switch r.URL.Path {
        case "/api/v1/users/current":
            _, _ = w.Write([]byte(`{"id":"user-1","name":"Ashish","email":"ashish@example.com","organisation_id":"org-1"}`))
        case "/api/v1/organizations/org-1/projects/default/stacks":
            _, _ = w.Write([]byte(`{"items":[{"id":"stack-1","name":"one","spec":{}},{"id":"stack-2","name":"two","spec":{}}],"total":2}`))
        default:
            t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
            w.WriteHeader(http.StatusNotFound)
        }
    }))
    defer server.Close()

    configPath := filepath.Join(t.TempDir(), "config.json")
    body := fmt.Sprintf(`{"server_url":%q,"access_token":"sdm_ctx_test","organization_id":"org-1","project_name":"default","current_stack":%q}`, server.URL, current)
    if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
        t.Fatal(err)
    }
    t.Setenv("STACKDOME_CONFIG", configPath)

    var stdoutBuffer, stderrBuffer bytes.Buffer
    code = runWithWriters([]string{"ctx", "-o", format}, &stdoutBuffer, &stderrBuffer)
    return stdoutBuffer.String(), stderrBuffer.String(), code
}

func decodeCtxOutput(t *testing.T, format, raw string, target any) {
    t.Helper()
    var err error
    if format == "yaml" {
        err = yaml.Unmarshal([]byte(raw), target)
    } else {
        err = json.Unmarshal([]byte(raw), target)
    }
    if err != nil {
        t.Fatalf("decode ctx %s: %v\n%s", format, err, raw)
    }
}

func TestCtxStructuredOutputResolvesSelectedStack(t *testing.T) {
    for _, format := range []string{"json", "yaml"} {
        stdout, stderr, code := executeCtx(t, format, "stack-2")
        if code != 0 || stderr != "" {
            t.Fatalf("ctx exit=%d stderr=%q", code, stderr)
        }
        var got ctxTestResult
        decodeCtxOutput(t, format, stdout, &got)
        if got.User != "Ashish" || got.ServerURL == "" || got.OrganizationID != "org-1" || got.Project != "default" {
            t.Fatalf("context = %#v", got)
        }
        if got.CurrentStack == nil || got.CurrentStack.Name != "two" || got.CurrentStack.ID != "stack-2" {
            t.Fatalf("current stack = %#v", got.CurrentStack)
        }
        if got.AuthMethod != "api token" || got.TokenSource != "config file" {
            t.Fatalf("auth = %q from %q", got.AuthMethod, got.TokenSource)
        }
    }
}

func TestCtxTableOutputIncludesContextAndResolvedStack(t *testing.T) {
    stdout, stderr, code := executeCtx(t, "table", "two")
    if code != 0 || stderr != "" {
        t.Fatalf("ctx exit=%d stderr=%q", code, stderr)
    }
    for _, want := range []string{"User:", "Ashish", "Server:", "Org:", "Project:", "Stack:", "two (stack-2)", "Auth:", "api token"} {
        if !strings.Contains(stdout, want) {
            t.Fatalf("table output omitted %q:\n%s", want, stdout)
        }
    }
}
```

- [ ] **Step 4: Write failing no-selection and stale-selection tests**

```go
func TestCtxWithoutSelectedStackSucceedsAndShowsGuidance(t *testing.T) {
    stdout, stderr, code := executeCtx(t, "table", "")
    if code != 0 || stderr != "" {
        t.Fatalf("ctx exit=%d stderr=%q", code, stderr)
    }
    if !strings.Contains(stdout, "Stack:    none") || !strings.Contains(stdout, "stackdome use stack <stack>") {
        t.Fatalf("missing no-stack guidance:\n%s", stdout)
    }
}

func TestCtxJSONOmitsCurrentStackWhenNoneSelected(t *testing.T) {
    stdout, _, code := executeCtx(t, "json", "")
    if code != 0 || strings.Contains(stdout, "current_stack") {
        t.Fatalf("ctx exit=%d output=%s", code, stdout)
    }
}

func TestCtxReturnsNotFoundForStaleSelection(t *testing.T) {
    _, stderr, code := executeCtx(t, "json", "missing-stack")
    if code != 3 || !strings.Contains(stderr, `Stack "missing-stack" not found`) {
        t.Fatalf("ctx exit=%d stderr=%q", code, stderr)
    }
}

func TestCtxNeverPrintsConfiguredToken(t *testing.T) {
    stdout, stderr, _ := executeCtx(t, "json", "stack-2")
    if strings.Contains(stdout, "sdm_ctx_test") || strings.Contains(stderr, "sdm_ctx_test") {
        t.Fatalf("ctx leaked configured token: stdout=%q stderr=%q", stdout, stderr)
    }
}
```

Add an environment-override regression test proving `config.Load` does not
leak a file-context selection into an environment-controlled context:

```go
func TestCtxEnvironmentOverrideDoesNotExposeFileStack(t *testing.T) {
    stackRequests := 0
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        switch r.URL.Path {
        case "/api/v1/users/current":
            _, _ = w.Write([]byte(`{"id":"user-1","name":"Ashish","email":"ashish@example.com","organisation_id":"env-org"}`))
        case "/api/v1/organizations/env-org/projects/env-project/stacks":
            stackRequests++
            _, _ = w.Write([]byte(`{"items":[],"total":0}`))
        default:
            t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
            w.WriteHeader(http.StatusNotFound)
        }
    }))
    defer server.Close()

    configPath := filepath.Join(t.TempDir(), "config.json")
    body := fmt.Sprintf(`{"server_url":%q,"access_token":"file-token","organization_id":"file-org","project_name":"file-project","current_stack":"file-stack"}`, server.URL)
    if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
        t.Fatal(err)
    }
    t.Setenv("STACKDOME_CONFIG", configPath)
    t.Setenv("STACKDOME_URL", server.URL)
    t.Setenv("STACKDOME_TOKEN", "sdm_env_test")
    t.Setenv("STACKDOME_ORG", "env-org")
    t.Setenv("STACKDOME_PROJECT", "env-project")

    var stdout, stderr bytes.Buffer
    code := runWithWriters([]string{"ctx", "-o", "json"}, &stdout, &stderr)
    if code != 0 || stderr.Len() != 0 {
        t.Fatalf("ctx exit=%d stderr=%q", code, stderr.String())
    }
    if strings.Contains(stdout.String(), "current_stack") || strings.Contains(stdout.String(), "file-stack") {
        t.Fatalf("ctx leaked file-backed stack: %s", stdout.String())
    }
    if stackRequests != 0 {
        t.Fatalf("stack-list requests = %d, want zero without an active selection", stackRequests)
    }
}
```

- [ ] **Step 5: Run context-output tests and verify RED**

Run:

```bash
go test ./cmd/stackdome -run TestCtx -count=1
```

Expected: FAIL because `newCtxCmd`, `ctxStack`, and context output do not exist.

- [ ] **Step 6: Implement shared auth details**

In `whoami.go`, add:

```go
func authDetails(cfg *config.Config) (method, source string) {
    method = "session (jwt)"
    if strings.HasPrefix(cfg.AccessToken, "sdm_") {
        method = "api token"
    }
    source = "config file"
    if cfg.TokenFromEnv() {
        source = "STACKDOME_TOKEN"
    }
    return method, source
}
```

Replace `whoami`'s inline method/source calculation with this helper without changing its output fields or labels.

- [ ] **Step 7: Implement `ctx.go`**

Create these output types and command:

```go
type ctxStack struct {
    Name string `json:"name" yaml:"name"`
    ID   string `json:"id" yaml:"id"`
}

type ctxInfo struct {
    User           string    `json:"user" yaml:"user"`
    Email          string    `json:"email,omitempty" yaml:"email,omitempty"`
    ServerURL      string    `json:"server_url" yaml:"server_url"`
    OrganizationID string    `json:"organization_id" yaml:"organization_id"`
    Project        string    `json:"project,omitempty" yaml:"project,omitempty"`
    CurrentStack   *ctxStack `json:"current_stack,omitempty" yaml:"current_stack,omitempty"`
    AuthMethod     string    `json:"auth_method" yaml:"auth_method"`
    TokenSource    string    `json:"token_source" yaml:"token_source"`
}

func newCtxCmd() *cobra.Command {
    return &cobra.Command{
        Use:     "ctx",
        Short:   "Show the active Stackdome context",
        Long:    "Show the authenticated user, Stackdome server, organization, project, selected stack, and authentication source without revealing credentials.",
        Example: "  stackdome ctx\n  stackdome ctx -o json",
        Args:    cobra.NoArgs,
        RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, _ []string) error {
            user, err := ctx.Client.GetCurrentUser(cmd.Context())
            if err != nil {
                return err
            }
            info := ctxInfo{
                User: userDisplayName(user), Email: user.GetEmail(),
                ServerURL: ctx.Config.ServerURL, OrganizationID: ctx.Config.OrganizationID,
                Project: ctx.Config.ProjectName,
            }
            info.AuthMethod, info.TokenSource = authDetails(ctx.Config)
            if ctx.Config.CurrentStack != "" {
                stacks, err := ctx.Client.ListStacks(cmd.Context())
                if err != nil {
                    return err
                }
                for i := range stacks {
                    if currentStackMatches(stacks[i], ctx.Config.CurrentStack) {
                        info.CurrentStack = &ctxStack{Name: stacks[i].Name, ID: stacks[i].GetId()}
                        break
                    }
                }
                if info.CurrentStack == nil {
                    return clierrors.NotFoundError("Stack", ctx.Config.CurrentStack)
                }
            }
            if !ctx.Formatter.IsTable() {
                return ctx.Formatter.PrintStructured(info)
            }
            renderCtx(ctx.Formatter, info)
            return nil
        })),
    }
}
```

Implement `renderCtx` using `Formatter.Printf`. Print user plus email only when distinct, then server, org, optional project, selected stack or `none`, no-stack guidance, and auth method/source in the approved order.

- [ ] **Step 8: Register and group `ctx`**

In `root.go`:

```go
rootCmd.AddCommand(newCtxCmd())
```

In `configureRootHelpGroups`:

```go
"ctx": "auth",
```

The command defines complete `Use`, `Short`, `Long`, and `Example` itself, so it satisfies the agent-help contract.

- [ ] **Step 9: Run focused tests and verify GREEN**

Run:

```bash
go test ./cmd/stackdome -run 'TestCtx|TestExecutableCommandsHaveAgentReadableHelp|TestRootCommandsAreGroupedByPurpose|TestLifecycleCommandsRejectUnexpectedPositionalArguments' -count=1
```

Expected: PASS.

- [ ] **Step 10: Verify `whoami` remains compatible**

Run:

```bash
go test ./cmd/stackdome -run 'Test.*Whoami|TestCtx' -count=1
```

Expected: PASS with unchanged `whoami` table and structured fields.

- [ ] **Step 11: Commit the context command**

```bash
git add cmd/stackdome/ctx.go cmd/stackdome/ctx_test.go cmd/stackdome/whoami.go cmd/stackdome/root.go cmd/stackdome/command_tree.go cmd/stackdome/command_tree_test.go
git commit -m "feat: add CLI context summary"
```

---

### Task 3: Full Verification and Delivery

**Files:**
- Verify only: all changed files

**Interfaces:**
- Consumes: completed Task 1 and Task 2 behavior.
- Produces: a verified feature branch and pull request.

- [ ] **Step 1: Format changed Go files**

```bash
gofmt -w cmd/stackdome/ctx.go cmd/stackdome/ctx_test.go cmd/stackdome/stack.go cmd/stackdome/stack_test.go cmd/stackdome/whoami.go cmd/stackdome/root.go cmd/stackdome/command_tree.go cmd/stackdome/command_tree_test.go
```

- [ ] **Step 2: Run the complete test suite**

```bash
go test ./... -count=1
```

Expected: every package reports `ok` and the command exits 0.

- [ ] **Step 3: Run static analysis and build**

```bash
go vet ./...
make build
```

Expected: both commands exit 0; the built binary reports the current feature commit.

- [ ] **Step 4: Smoke-test built command discovery**

```bash
./bin/stackdome ctx --help
./bin/stackdome get stacks --help
./bin/stackdome list stacks --help
```

Expected: all commands exit 0, `ctx` documents every output field, and stack-list help remains intact.

- [ ] **Step 5: Inspect the final diff and repository state**

```bash
git diff main...HEAD --check
git diff main...HEAD --stat
git status --short
```

Expected: no whitespace errors and no uncommitted changes.

- [ ] **Step 6: Push and open a PR**

```bash
git push -u origin feat/cli-context
gh pr create --base main --head feat/cli-context --title "feat: add CLI context summary" --body-file -
```

The PR body summarizes `ctx`, the `CURRENT` table column, structured `current` flags, compatibility behavior, and the exact verification commands.
