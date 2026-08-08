# Windows Installer Module-Path Test Fix

## Context

PR #5 runs the Windows installer integration tests from a GitHub Actions step whose host shell is PowerShell 7. The Go test process inherits that shell's `PSModulePath` and launches both `pwsh` and Windows PowerShell 5.1 (`powershell.exe`) as child processes.

The PowerShell 5.1 child inherits the PowerShell 7 module path because Go starts it directly. It therefore cannot auto-load the Windows PowerShell copy of `Microsoft.PowerShell.Utility`, and the installer fails when it calls `Get-FileHash`. The same installer test passes under PowerShell 7.

## Design

Keep the installer unchanged. Correct the test harness at the process boundary where it creates a Windows PowerShell child:

- Build the child environment through a shell-aware helper.
- For `powershell.exe` only, omit the inherited `PSModulePath` variable so Windows PowerShell constructs its native default module search path at startup.
- Preserve all other environment variables and the existing Stackdome-specific overrides.
- Leave the environment unchanged for `pwsh` apart from the existing overrides.

This keeps the test representative of a normal Windows PowerShell invocation and avoids weakening checksum verification or replacing other PowerShell module commands in the production installer.

## Testing

Use test-driven development for the pure environment-building behavior:

1. Add a test proving a case-insensitive inherited `PSModulePath` is absent for `powershell.exe` while unrelated variables and overrides remain intact.
2. Add or retain coverage proving `PSModulePath` remains available to `pwsh`.
3. Run the focused Go tests locally; Windows-only installer execution will remain skipped on non-Windows systems.
4. Run the complete Go test suite, race suite, vet, installer syntax check, and build.
5. Push the fix so GitHub Actions reruns the real PowerShell 5.1 and PowerShell 7 integration tests on Windows.

## Scope

Only the Windows installer test harness changes. The installer, release workflow, and CLI behavior remain unchanged.
