package installunix_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

const testVersion = "v1.2.3"

func TestInstallsVersionedAssetForSupportedPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		unameOS  string
		unameCPU string
		wantOS   string
		wantArch string
	}{
		{name: "macOS Intel", unameOS: "Darwin", unameCPU: "x86_64", wantOS: "darwin", wantArch: "amd64"},
		{name: "macOS Apple Silicon", unameOS: "Darwin", unameCPU: "arm64", wantOS: "darwin", wantArch: "arm64"},
		{name: "Linux Intel", unameOS: "Linux", unameCPU: "x86_64", wantOS: "linux", wantArch: "amd64"},
		{name: "Linux ARM", unameOS: "Linux", unameCPU: "aarch64", wantOS: "linux", wantArch: "arm64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := fmt.Sprintf("stackdome_%s_%s_%s.tar.gz", testVersion, tt.wantOS, tt.wantArch)
			binary := []byte("#!/bin/sh\nprintf 'installed test binary\\n'\n")
			archive := makeArchive(t, binary)
			digest := sha256.Sum256(archive)
			checksums := fmt.Sprintf("%x  %s\n", digest, asset)

			var requestMu sync.Mutex
			var requested []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestMu.Lock()
				requested = append(requested, r.URL.Path)
				requestMu.Unlock()
				switch r.URL.Path {
				case "/downloads/" + testVersion + "/" + asset:
					_, _ = w.Write(archive)
				case "/downloads/" + testVersion + "/checksums.txt":
					_, _ = io.WriteString(w, checksums)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)

			installDir := t.TempDir()
			result := runInstaller(t, installerEnv{
				installDir: installDir,
				version:    testVersion,
				releaseURL: server.URL + "/downloads",
				unameOS:    tt.unameOS,
				unameCPU:   tt.unameCPU,
			})
			if result.err != nil {
				t.Fatalf("installer failed: %v\noutput:\n%s", result.err, result.output)
			}

			installed, err := os.ReadFile(filepath.Join(installDir, "stackdome"))
			if err != nil {
				t.Fatalf("read installed binary: %v", err)
			}
			if !bytes.Equal(installed, binary) {
				t.Fatalf("installed binary = %q, want %q", installed, binary)
			}
			info, err := os.Stat(filepath.Join(installDir, "stackdome"))
			if err != nil {
				t.Fatalf("stat installed binary: %v", err)
			}
			if info.Mode()&0o111 == 0 {
				t.Fatalf("installed binary mode %v is not executable", info.Mode())
			}
			wantWarning := "warning: " + installDir + " is not on your PATH; add it before running stackdome"
			if !strings.Contains(result.output, wantWarning) {
				t.Fatalf("output %q does not contain PATH warning %q", result.output, wantWarning)
			}

			wantRequests := []string{
				"/downloads/" + testVersion + "/" + asset,
				"/downloads/" + testVersion + "/checksums.txt",
			}
			requestMu.Lock()
			defer requestMu.Unlock()
			if strings.Join(requested, "\n") != strings.Join(wantRequests, "\n") {
				t.Fatalf("requested paths = %q, want %q", requested, wantRequests)
			}
		})
	}
}

func TestDiscoversLatestReleaseWithoutJQ(t *testing.T) {
	const repository = "example/stackdome-cli"
	asset := "stackdome_v9.8.7_linux_arm64.tar.gz"
	binary := []byte("latest release binary\n")
	archive := makeArchive(t, binary)
	digest := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%x  %s\n", digest, asset)

	var requestMu sync.Mutex
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		requested = append(requested, r.URL.Path)
		requestMu.Unlock()
		switch r.URL.Path {
		case "/api/repos/" + repository + "/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, "{\n  \"tag_name\": \"v9.8.7\",\n  \"name\": \"Stackdome v9.8.7\"\n}\n")
		case "/downloads/v9.8.7/" + asset:
			_, _ = w.Write(archive)
		case "/downloads/v9.8.7/checksums.txt":
			_, _ = io.WriteString(w, checksums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	installDir := t.TempDir()
	result := runInstaller(t, installerEnv{
		installDir: installDir,
		releaseURL: server.URL + "/downloads",
		apiURL:     server.URL + "/api",
		repository: repository,
		unameOS:    "Linux",
		unameCPU:   "aarch64",
	})
	if result.err != nil {
		t.Fatalf("installer failed: %v\noutput:\n%s", result.err, result.output)
	}

	installed, err := os.ReadFile(filepath.Join(installDir, "stackdome"))
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if !bytes.Equal(installed, binary) {
		t.Fatalf("installed binary = %q, want %q", installed, binary)
	}
	wantRequests := []string{
		"/api/repos/" + repository + "/releases/latest",
		"/downloads/v9.8.7/" + asset,
		"/downloads/v9.8.7/checksums.txt",
	}
	requestMu.Lock()
	defer requestMu.Unlock()
	if strings.Join(requested, "\n") != strings.Join(wantRequests, "\n") {
		t.Fatalf("requested paths = %q, want %q", requested, wantRequests)
	}
}

func TestRejectsChecksumMismatchAndCleansTemporaryFiles(t *testing.T) {
	asset := "stackdome_v1.2.3_linux_amd64.tar.gz"
	archive := makeArchive(t, []byte("must not be installed\n"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/downloads/v1.2.3/" + asset:
			_, _ = w.Write(archive)
		case "/downloads/v1.2.3/checksums.txt":
			_, _ = io.WriteString(w, strings.Repeat("0", 64)+"  "+asset+"\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	installDir := t.TempDir()
	temporaryRoot := t.TempDir()
	result := runInstaller(t, installerEnv{
		installDir: installDir,
		tmpDir:     temporaryRoot,
		version:    testVersion,
		releaseURL: server.URL + "/downloads",
		unameOS:    "Linux",
		unameCPU:   "x86_64",
	})
	if result.err == nil {
		t.Fatalf("installer succeeded with mismatched checksum; output:\n%s", result.output)
	}
	if !strings.Contains(result.output, "checksum verification failed") {
		t.Fatalf("output %q does not explain checksum failure", result.output)
	}
	if _, err := os.Stat(filepath.Join(installDir, "stackdome")); !os.IsNotExist(err) {
		t.Fatalf("stackdome was installed after checksum failure (stat err: %v)", err)
	}
	entries, err := os.ReadDir(temporaryRoot)
	if err != nil {
		t.Fatalf("read temporary root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files were not cleaned up: %v", entries)
	}
}

func TestRequiresExactChecksumFilename(t *testing.T) {
	asset := "stackdome_v1.2.3_linux_amd64.tar.gz"
	archive := makeArchive(t, []byte("binary with checksum under wrong name\n"))
	digest := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/downloads/v1.2.3/" + asset:
			_, _ = w.Write(archive)
		case "/downloads/v1.2.3/checksums.txt":
			_, _ = fmt.Fprintf(w, "%x  %s.backup\n", digest, asset)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	installDir := t.TempDir()
	result := runInstaller(t, installerEnv{
		installDir: installDir,
		version:    testVersion,
		releaseURL: server.URL + "/downloads",
		unameOS:    "Linux",
		unameCPU:   "x86_64",
	})
	if result.err == nil {
		t.Fatalf("installer accepted checksum for a different filename; output:\n%s", result.output)
	}
	if !strings.Contains(result.output, "does not contain an exact entry for "+asset) {
		t.Fatalf("output %q does not explain the missing exact checksum entry", result.output)
	}
}

func TestRejectsUnsupportedPlatformsWithActionableError(t *testing.T) {
	tests := []struct {
		name     string
		unameOS  string
		unameCPU string
		want     string
	}{
		{name: "operating system", unameOS: "FreeBSD", unameCPU: "x86_64", want: "unsupported operating system: FreeBSD (supported: macOS and Linux)"},
		{name: "architecture", unameOS: "Linux", unameCPU: "ppc64le", want: "unsupported architecture: ppc64le (supported: amd64 and arm64)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runInstaller(t, installerEnv{
				installDir: t.TempDir(),
				version:    testVersion,
				unameOS:    tt.unameOS,
				unameCPU:   tt.unameCPU,
			})
			if result.err == nil {
				t.Fatalf("installer succeeded on unsupported platform; output:\n%s", result.output)
			}
			if !strings.Contains(result.output, tt.want) {
				t.Fatalf("output %q does not contain %q", result.output, tt.want)
			}
		})
	}
}

func TestDownloadFailureDoesNotLogReleaseURLSecrets(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)

	const secret = "token-super-secret"
	result := runInstaller(t, installerEnv{
		installDir: t.TempDir(),
		version:    testVersion,
		releaseURL: server.URL + "/" + secret,
		unameOS:    "Linux",
		unameCPU:   "x86_64",
	})
	if result.err == nil {
		t.Fatalf("installer succeeded without a release asset; output:\n%s", result.output)
	}
	if strings.Contains(result.output, secret) {
		t.Fatalf("installer logged secret-bearing release URL: %q", result.output)
	}
	if !strings.Contains(result.output, "could not download release asset stackdome_v1.2.3_linux_amd64.tar.gz") {
		t.Fatalf("output %q does not contain an actionable download error", result.output)
	}
}

func TestMalformedLatestResponseSuggestsVersionOverrideWithoutLoggingAPIURL(t *testing.T) {
	const secret = "api-token-super-secret"
	const repository = "example/stackdome-cli"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+secret+"/repos/"+repository+"/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\"message\":\"unexpected response\"}\n")
	}))
	t.Cleanup(server.Close)

	result := runInstaller(t, installerEnv{
		installDir: t.TempDir(),
		apiURL:     server.URL + "/" + secret,
		repository: repository,
		unameOS:    "Linux",
		unameCPU:   "x86_64",
	})
	if result.err == nil {
		t.Fatalf("installer succeeded with malformed latest-release response; output:\n%s", result.output)
	}
	if !strings.Contains(result.output, "set STACKDOME_VERSION explicitly") {
		t.Fatalf("output %q does not suggest the version override", result.output)
	}
	if strings.Contains(result.output, secret) {
		t.Fatalf("installer logged secret-bearing API URL: %q", result.output)
	}
}

type installerEnv struct {
	installDir string
	tmpDir     string
	version    string
	releaseURL string
	apiURL     string
	repository string
	unameOS    string
	unameCPU   string
}

type commandResult struct {
	output string
	err    error
}

func runInstaller(t *testing.T, env installerEnv) commandResult {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer test")
	}

	script, err := filepath.Abs(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatalf("resolve installer path: %v", err)
	}
	fakeBin := t.TempDir()
	unamePath := filepath.Join(fakeBin, "uname")
	unameScript := "#!/bin/sh\ncase \"$1\" in\n  -s) printf '%s\\n' \"$TEST_UNAME_S\" ;;\n  -m) printf '%s\\n' \"$TEST_UNAME_M\" ;;\n  *) exit 2 ;;\nesac\n"
	if err := os.WriteFile(unamePath, []byte(unameScript), 0o755); err != nil {
		t.Fatalf("write uname shim: %v", err)
	}
	jqPath := filepath.Join(fakeBin, "jq")
	if err := os.WriteFile(jqPath, []byte("#!/bin/sh\nexit 97\n"), 0o755); err != nil {
		t.Fatalf("write jq shim: %v", err)
	}

	path := fakeBin + string(os.PathListSeparator) + os.Getenv("PATH")
	shell := os.Getenv("STACKDOME_TEST_SHELL")
	if shell == "" {
		shell = "sh"
	}
	cmd := exec.Command(shell, script)
	cmd.Env = []string{
		"PATH=" + path,
		"HOME=" + os.Getenv("HOME"),
		"STACKDOME_INSTALL_DIR=" + env.installDir,
		"STACKDOME_VERSION=" + env.version,
		"STACKDOME_RELEASE_BASE_URL=" + env.releaseURL,
		"STACKDOME_API_BASE_URL=" + env.apiURL,
		"STACKDOME_REPOSITORY=" + env.repository,
		"TMPDIR=" + env.tmpDir,
		"TEST_UNAME_S=" + env.unameOS,
		"TEST_UNAME_M=" + env.unameCPU,
	}
	output, runErr := cmd.CombinedOutput()
	return commandResult{output: string(output), err: runErr}
}

func makeArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "stackdome",
		Mode: 0o755,
		Size: int64(len(binary)),
	}); err != nil {
		t.Fatalf("write archive header: %v", err)
	}
	if _, err := tarWriter.Write(binary); err != nil {
		t.Fatalf("write archive body: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return output.Bytes()
}
