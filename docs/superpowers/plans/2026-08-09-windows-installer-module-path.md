# Windows Installer Module-Path Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make PR #5's Windows installer integration tests pass under both Windows PowerShell 5.1 and PowerShell 7 without changing production installer behavior.

**Architecture:** Correct the Go test harness at the child-process boundary. A shell-aware environment helper will omit an inherited PowerShell 7 `PSModulePath` only when launching `powershell.exe`, allowing Windows PowerShell to construct its native module path; `pwsh` and all unrelated variables retain their current behavior.

**Tech Stack:** Go 1.25, Windows PowerShell 5.1, PowerShell 7, GitHub Actions

## Global Constraints

- Only the Windows installer test harness changes.
- The installer, release workflow, and CLI behavior remain unchanged.
- Preserve every environment variable except `PSModulePath` when launching `powershell.exe`.
- Preserve the existing Stackdome-specific environment overrides for both shells.
- Match environment variable names case-insensitively, as required on Windows.

---

### Task 1: Make the test subprocess environment shell-aware

**Files:**
- Modify: `tests/install_windows/install_test.go:224-260`
- Test: `tests/install_windows/install_test.go`

**Interfaces:**
- Consumes: `base []string`, `shell string`, and `overrides map[string]string` from `runInstaller`.
- Produces: `environmentForShell(base []string, shell string, overrides map[string]string) []string`, used as `exec.Cmd.Env`.

- [ ] **Step 1: Write failing environment tests**

Add pure, cross-platform tests before changing the helper:

```go
func TestEnvironmentForShellDropsInheritedPSModulePathForWindowsPowerShell(t *testing.T) {
	base := []string{
		"KEEP=original",
		"PsMoDuLePaTh=C:\\Program Files\\PowerShell\\7\\Modules",
		"STACKDOME_ARCH=old",
	}

	environment := environmentForShell(base, "powershell.exe", map[string]string{
		"STACKDOME_ARCH": "arm64",
	})

	if _, found := environmentValue(environment, "PSModulePath"); found {
		t.Fatalf("PSModulePath remained in Windows PowerShell environment: %q", environment)
	}
	if value, found := environmentValue(environment, "KEEP"); !found || value != "original" {
		t.Fatalf("KEEP = %q, %v; want original, true", value, found)
	}
	if value, found := environmentValue(environment, "STACKDOME_ARCH"); !found || value != "arm64" {
		t.Fatalf("STACKDOME_ARCH = %q, %v; want arm64, true", value, found)
	}
}

func TestEnvironmentForShellPreservesPSModulePathForPowerShell7(t *testing.T) {
	base := []string{"PSModulePath=C:\\Program Files\\PowerShell\\7\\Modules"}

	environment := environmentForShell(base, "pwsh.exe", nil)

	value, found := environmentValue(environment, "PSModulePath")
	if !found || value != `C:\Program Files\PowerShell\7\Modules` {
		t.Fatalf("PSModulePath = %q, %v; want PowerShell 7 path, true", value, found)
	}
}

func environmentValue(environment []string, name string) (string, bool) {
	for _, entry := range environment {
		entryName, value, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(entryName, name) {
			return value, true
		}
	}
	return "", false
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
go test ./tests/install_windows -run TestEnvironmentForShell -count=1 -v
```

Expected: build failure containing `undefined: environmentForShell`.

- [ ] **Step 3: Implement the minimal shell-aware helper**

Change `runInstaller` to call:

```go
cmd.Env = environmentForShell(os.Environ(), shell, testEnvironment)
```

Replace `environmentWithOverrides` with:

```go
func environmentForShell(base []string, shell string, overrides map[string]string) []string {
	environment := make([]string, 0, len(base)+len(overrides))
	dropModulePath := strings.EqualFold(filepath.Base(shell), "powershell.exe")
	for _, entry := range base {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if dropModulePath && strings.EqualFold(name, "PSModulePath") {
			continue
		}
		if _, overridden := lookupFold(overrides, name); !overridden {
			environment = append(environment, entry)
		}
	}
	for name, value := range overrides {
		environment = append(environment, name+"="+value)
	}
	return environment
}
```

- [ ] **Step 4: Format and verify GREEN**

Run:

```bash
gofmt -w tests/install_windows/install_test.go
go test ./tests/install_windows -run TestEnvironmentForShell -count=1 -v
```

Expected: both environment tests pass.

- [ ] **Step 5: Run repository verification**

Run:

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
sh -n install.sh
go build ./cmd/stackdome
actionlint .github/workflows/*.yml
```

Expected: every command exits with status 0. On non-Windows systems, the installer execution tests skip while the new pure environment tests run.

- [ ] **Step 6: Commit the test-harness fix**

```bash
git add tests/install_windows/install_test.go
git commit -m "fix: isolate Windows PowerShell module path in tests"
```

### Task 2: Update and verify PR #5

**Files:**
- No additional files.

**Interfaces:**
- Consumes: the committed test-harness fix from Task 1.
- Produces: an updated `feat/cli-installers` branch with successful GitHub Actions checks.

- [ ] **Step 1: Push the branch**

Run:

```bash
git push origin feat/cli-installers
```

Expected: the remote branch advances to the local fix commit and PR #5 starts new CI runs.

- [ ] **Step 2: Monitor the PR checks**

Query the check runs for the new head commit until both `Test CLI and installers` and `PowerShell 5.1 and PowerShell 7` complete.

Expected: both checks conclude with `success`. If the Windows check still fails, retrieve its complete job log and return to root-cause investigation before making another change.
