package installwindows_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const windowsTestVersion = "v1.2.3"

func TestInstallsARM64ArchiveWhenX64PowerShellIsEmulated(t *testing.T) {
	script := readInstaller(t)
	assetName := "stackdome_" + windowsTestVersion + "_windows_arm64.zip"
	binary := []byte("fake Windows ARM64 executable\r\n")
	archive := makeZipArchive(t, binary)
	digest := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%x  %s\n", digest, assetName)

	var requestMu sync.Mutex
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		requested = append(requested, r.URL.Path)
		requestMu.Unlock()

		switch r.URL.Path {
		case "/install.ps1":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write(script)
		case "/downloads/" + windowsTestVersion + "/" + assetName:
			_, _ = w.Write(archive)
		case "/downloads/" + windowsTestVersion + "/checksums.txt":
			_, _ = io.WriteString(w, checksums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	for _, shell := range powershells(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			installDir := t.TempDir()
			result := runInstaller(t, shell, server.URL+"/install.ps1", map[string]string{
				"STACKDOME_VERSION":          windowsTestVersion,
				"STACKDOME_INSTALL_DIR":      installDir,
				"STACKDOME_RELEASE_BASE_URL": server.URL + "/downloads",
				"STACKDOME_ARCH":             "",
				"STACKDOME_SKIP_PATH_UPDATE": "1",
				"PROCESSOR_ARCHITEW6432":     "ARM64",
				"PROCESSOR_ARCHITECTURE":     "AMD64",
			})
			if result.err != nil {
				t.Fatalf("installer failed: %v\noutput:\n%s", result.err, result.output)
			}

			installed, err := os.ReadFile(filepath.Join(installDir, "stackdome.exe"))
			if err != nil {
				t.Fatalf("read installed executable: %v", err)
			}
			if !bytes.Equal(installed, binary) {
				t.Fatalf("installed executable = %q, want %q", installed, binary)
			}
		})
	}

	requestMu.Lock()
	defer requestMu.Unlock()
	joinedRequests := strings.Join(requested, "\n")
	for _, path := range []string{
		"/downloads/" + windowsTestVersion + "/" + assetName,
		"/downloads/" + windowsTestVersion + "/checksums.txt",
	} {
		if !strings.Contains(joinedRequests, path) {
			t.Errorf("installer did not request %s; requests:\n%s", path, joinedRequests)
		}
	}
}

func TestRejectsChecksumMismatch(t *testing.T) {
	script := readInstaller(t)
	assetName := "stackdome_" + windowsTestVersion + "_windows_amd64.zip"
	archive := makeZipArchive(t, []byte("must not be installed\r\n"))
	checksums := strings.Repeat("0", 64) + "  " + assetName + "\n"

	server := newInstallerServer(t, script, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/downloads/" + windowsTestVersion + "/" + assetName:
			_, _ = w.Write(archive)
		case "/downloads/" + windowsTestVersion + "/checksums.txt":
			_, _ = io.WriteString(w, checksums)
		default:
			http.NotFound(w, r)
		}
	})

	for _, shell := range powershells(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			installDir := t.TempDir()
			result := runInstaller(t, shell, server.URL+"/install.ps1", map[string]string{
				"STACKDOME_VERSION":          windowsTestVersion,
				"STACKDOME_INSTALL_DIR":      installDir,
				"STACKDOME_RELEASE_BASE_URL": server.URL + "/downloads",
				"STACKDOME_ARCH":             "amd64",
				"STACKDOME_SKIP_PATH_UPDATE": "1",
			})
			if result.err == nil {
				t.Fatalf("installer accepted a mismatched checksum; output:\n%s", result.output)
			}
			if !strings.Contains(result.output, "Checksum mismatch") {
				t.Fatalf("output %q does not explain the checksum mismatch", result.output)
			}
			if _, err := os.Stat(filepath.Join(installDir, "stackdome.exe")); !os.IsNotExist(err) {
				t.Fatalf("executable was installed after checksum mismatch (stat error: %v)", err)
			}
		})
	}
}

func TestDownloadFailureDoesNotExposeReleaseURL(t *testing.T) {
	script := readInstaller(t)
	const secret = "token-super-secret"
	server := newInstallerServer(t, script, http.NotFoundHandler().ServeHTTP)

	for _, shell := range powershells(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			result := runInstaller(t, shell, server.URL+"/install.ps1", map[string]string{
				"STACKDOME_VERSION":          windowsTestVersion,
				"STACKDOME_INSTALL_DIR":      t.TempDir(),
				"STACKDOME_RELEASE_BASE_URL": server.URL + "/" + secret,
				"STACKDOME_ARCH":             "amd64",
				"STACKDOME_SKIP_PATH_UPDATE": "1",
			})
			if result.err == nil {
				t.Fatalf("installer succeeded without a release asset; output:\n%s", result.output)
			}
			if strings.Contains(result.output, secret) {
				t.Fatalf("installer exposed a secret-bearing release URL: %q", result.output)
			}
			if !strings.Contains(result.output, "Unable to download stackdome_v1.2.3_windows_amd64.zip") {
				t.Fatalf("output %q does not identify the unavailable asset", result.output)
			}
		})
	}
}

func TestMalformedLatestResponseIsActionableAndRedacted(t *testing.T) {
	script := readInstaller(t)
	const secret = "api-token-super-secret"
	const repository = "example/stackdome-cli"
	server := newInstallerServer(t, script, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+secret+"/repos/"+repository+"/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{}\n")
	})

	for _, shell := range powershells(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			result := runInstaller(t, shell, server.URL+"/install.ps1", map[string]string{
				"STACKDOME_VERSION":          "",
				"STACKDOME_INSTALL_DIR":      t.TempDir(),
				"STACKDOME_API_BASE_URL":     server.URL + "/" + secret,
				"STACKDOME_RELEASE_BASE_URL": server.URL + "/downloads",
				"STACKDOME_REPOSITORY":       repository,
				"STACKDOME_ARCH":             "amd64",
				"STACKDOME_SKIP_PATH_UPDATE": "1",
			})
			if result.err == nil {
				t.Fatalf("installer accepted malformed latest-release metadata; output:\n%s", result.output)
			}
			if strings.Contains(result.output, secret) {
				t.Fatalf("installer exposed a secret-bearing API URL: %q", result.output)
			}
			if !strings.Contains(result.output, "Set STACKDOME_VERSION explicitly") {
				t.Fatalf("output %q does not suggest the version override", result.output)
			}
			if strings.Contains(result.output, "Property 'tag_name' cannot be found") {
				t.Fatalf("strict-mode property error escaped instead of actionable remediation: %q", result.output)
			}
		})
	}
}

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

type commandResult struct {
	output string
	err    error
}

func powershells(t *testing.T) []string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell execution test")
	}

	var shells []string
	for _, name := range []string{"powershell.exe", "pwsh"} {
		path, err := exec.LookPath(name)
		if err == nil {
			shells = append(shells, path)
		}
	}
	if os.Getenv("STACKDOME_REQUIRE_BOTH_POWERSHELLS") == "1" && len(shells) != 2 {
		t.Fatalf("Windows CI requires both powershell.exe 5.1 and pwsh; found %d", len(shells))
	}
	if len(shells) == 0 {
		t.Skip("powershell.exe and pwsh are unavailable")
	}
	return shells
}

func runInstaller(t *testing.T, shell, scriptURL string, overrides map[string]string) commandResult {
	t.Helper()
	testEnvironment := map[string]string{
		"STACKDOME_VERSION":          "",
		"STACKDOME_INSTALL_DIR":      "",
		"STACKDOME_RELEASE_BASE_URL": "",
		"STACKDOME_API_BASE_URL":     "",
		"STACKDOME_REPOSITORY":       "Stackdome/stackdome-cli",
		"STACKDOME_ARCH":             "",
		"STACKDOME_SKIP_PATH_UPDATE": "1",
	}
	for name, value := range overrides {
		testEnvironment[name] = value
	}

	command := fmt.Sprintf("Invoke-RestMethod -Uri '%s' | Invoke-Expression", scriptURL)
	cmd := exec.Command(shell, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command)
	cmd.Env = environmentForShell(os.Environ(), shell, testEnvironment)
	output, err := cmd.CombinedOutput()
	return commandResult{output: string(output), err: err}
}

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

func lookupFold(values map[string]string, name string) (string, bool) {
	for key, value := range values {
		if strings.EqualFold(key, name) {
			return value, true
		}
	}
	return "", false
}

func readInstaller(t *testing.T) []byte {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "install.ps1"))
	if err != nil {
		t.Fatalf("resolve install.ps1: %v", err)
	}
	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}
	return script
}

func newInstallerServer(t *testing.T, script []byte, releaseHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/install.ps1" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write(script)
			return
		}
		releaseHandler(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

func makeZipArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	file, err := archive.Create("stackdome.exe")
	if err != nil {
		t.Fatalf("create executable in ZIP: %v", err)
	}
	if _, err := file.Write(binary); err != nil {
		t.Fatalf("write executable to ZIP: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return output.Bytes()
}
