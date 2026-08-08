package installer

import (
	"os"
	"strings"
	"testing"
)

func readWindowsInstaller(t *testing.T) string {
	t.Helper()

	script, err := os.ReadFile("install.ps1")
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}
	return string(script)
}

func requireInstallerContracts(t *testing.T, script string, contracts ...string) {
	t.Helper()

	for _, contract := range contracts {
		if !strings.Contains(script, contract) {
			t.Errorf("install.ps1 is missing contract %q", contract)
		}
	}
}

func TestWindowsInstallerSupportsReleaseAndTestOverrides(t *testing.T) {
	script := readWindowsInstaller(t)

	requireInstallerContracts(t, script,
		"STACKDOME_VERSION",
		"STACKDOME_INSTALL_DIR",
		"STACKDOME_REPOSITORY",
		"STACKDOME_RELEASE_BASE_URL",
		"STACKDOME_API_BASE_URL",
		"STACKDOME_ARCH",
		"api.github.com",
		"/repos/{1}/releases/latest",
		"Invalid release version",
	)

	for _, incompatible := range []string{"$PSScriptRoot", "Read-Host"} {
		if strings.Contains(script, incompatible) {
			t.Errorf("install.ps1 uses %q, which is incompatible with non-interactive `irm ... | iex` installation", incompatible)
		}
	}
}

func TestWindowsInstallerSelectsVersionedWindowsArchive(t *testing.T) {
	script := readWindowsInstaller(t)

	requireInstallerContracts(t, script,
		"PROCESSOR_ARCHITEW6432",
		"PROCESSOR_ARCHITECTURE",
		"[Runtime.InteropServices.RuntimeInformation]::OSArchitecture",
		"'X64' { 'amd64' }",
		"'ARM64' { 'arm64' }",
		"Unsupported Windows architecture",
		"stackdome_{0}_windows_{1}.zip",
		"checksums.txt",
	)
}

func TestWindowsInstallerDoesNotLogOverrideURLsOrNestedDownloadErrors(t *testing.T) {
	script := readWindowsInstaller(t)

	for _, leakedValue := range []string{
		"from $Uri",
		"from $latestReleaseUrl",
		"$($_.Exception.Message)",
	} {
		if strings.Contains(script, leakedValue) {
			t.Errorf("install.ps1 can expose credentials through error text %q", leakedValue)
		}
	}

	requireInstallerContracts(t, script,
		"Check network access or set STACKDOME_VERSION explicitly",
		"Verify that the release and asset exist, then retry",
	)
}

func TestWindowsInstallerVerifiesExactArchiveSHA256(t *testing.T) {
	script := readWindowsInstaller(t)

	requireInstallerContracts(t, script,
		"Get-FileHash",
		"-Algorithm SHA256",
		"[regex]::Escape($assetName)",
		"(?im)^",
		"Checksum mismatch",
		"Expand-Archive",
	)

	if strings.Index(script, "Get-FileHash") > strings.Index(script, "Expand-Archive") {
		t.Error("install.ps1 must verify the archive before extracting it")
	}
}

func TestWindowsInstallerUpdatesProcessAndPersistentUserPath(t *testing.T) {
	script := readWindowsInstaller(t)

	requireInstallerContracts(t, script,
		"LocalApplicationData",
		"Programs\\Stackdome\\bin",
		"STACKDOME_SKIP_PATH_UPDATE",
		"if ([string]::IsNullOrWhiteSpace($PathValue))",
		"[Environment]::GetEnvironmentVariable('Path', 'User')",
		"[Environment]::SetEnvironmentVariable('Path', $updatedUserPath, 'User')",
		"$env:Path",
		"Test-PathContains",
		"Restart your terminal",
	)
}

func TestWindowsInstallerHasRealWindowsExecutionCoverage(t *testing.T) {
	testSource, err := os.ReadFile("tests/install_windows/install_test.go")
	if err != nil {
		t.Fatalf("read Windows execution tests: %v", err)
	}
	workflow, err := os.ReadFile(".github/workflows/windows-installer.yml")
	if err != nil {
		t.Fatalf("read Windows installer workflow: %v", err)
	}

	requireInstallerContracts(t, string(testSource),
		"httptest.NewServer",
		"archive/zip",
		"powershell.exe",
		"pwsh",
		"STACKDOME_SKIP_PATH_UPDATE",
		"STACKDOME_REQUIRE_BOTH_POWERSHELLS",
	)
	requireInstallerContracts(t, string(workflow),
		"windows-latest",
		"go test ./tests/install_windows",
		"STACKDOME_REQUIRE_BOTH_POWERSHELLS",
	)
}
