package stackfile_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/stackfile"
	"github.com/Stackdome/stackdome/pkg/models"
	"gopkg.in/yaml.v3"
)

const singlePortSelfRef = `name: demo
resources:
  web:
    image: nginx:latest
    ports:
      - name: http
        port: 3000
        public: true
    env:
      PUBLIC_URL: "{{ self.public_url }}"
`

func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A single-port resource drops the port suffix: the self ref is
// `{{ self.public_url }}` and the emitted self_output must be one of the
// declared outputs the server validator builds from the same resource.
func TestSelfPublicURLSinglePort(t *testing.T) {
	sf, err := stackfile.Load(write(t, "stackfile.yaml", singlePortSelfRef))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	stack, err := sf.ToStack()
	if err != nil {
		t.Fatalf("to stack: %v", err)
	}

	res := stack.Spec.StackResources[0]
	env := res.ExecutionConfig.EnvironmentVariables[0]
	if env.Name != "PUBLIC_URL" || env.SelfOutput == nil {
		t.Fatalf("expected PUBLIC_URL with a self_output, got %+v", env)
	}
	if *env.SelfOutput != models.OutputNamePublicURL {
		t.Fatalf("self_output = %q, want %q", *env.SelfOutput, models.OutputNamePublicURL)
	}

	// Same check the server's stackresource validator runs: self_output must
	// appear in the resource's declared outputs.
	declared := models.StackResourceOutputDescriptors(&models.StackResource{
		Name:  res.Name,
		Ports: models.Ports{{Name: "http", Number: 3000, ExposedToPublic: true}},
	})
	for _, d := range declared {
		if d.Name == *env.SelfOutput {
			return
		}
	}
	t.Fatalf("self_output %q is not a declared output: %+v", *env.SelfOutput, declared)
}

// The old CLI spelling — it 400'd on deploy because the server never declares
// a `public.http.url` output for a single-port resource.
func TestSelfPublicURLOldSpellingRejected(t *testing.T) {
	bad := strings.Replace(singlePortSelfRef, "self.public_url", "self.public.http.url", 1)
	_, err := stackfile.Load(write(t, "stackfile.yaml", bad))
	if err == nil {
		t.Fatal("expected validation to reject 'self.public.http.url'")
	}
	if !strings.Contains(err.Error(), "public_url") {
		t.Fatalf("error should point at the valid outputs, got: %v", err)
	}
}

func TestLoadRejectsUnknownKeysIncludingCLIOnlyEnvFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "top-level typo",
			content: singlePortSelfRef + "nmae: typo\n",
		},
		{
			name:    "cli-only env_file",
			content: singlePortSelfRef + "    env_file: web.env\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := stackfile.Load(write(t, "stackfile.yaml", tt.content))
			if err == nil {
				t.Fatal("expected unknown key to be rejected")
			}
		})
	}
}

func TestLoadAllowsCommitWithBranchOrTagButNotBoth(t *testing.T) {
	tests := []struct {
		name    string
		build   string
		wantErr bool
	}{
		{
			name:  "branch and commit",
			build: "      repo: https://github.com/example/app.git\n      branch: main\n      commit: deadbeef\n",
		},
		{
			name:  "tag and commit",
			build: "      repo: https://github.com/example/app.git\n      tag: v1.0.0\n      commit: deadbeef\n",
		},
		{
			name:    "branch and tag",
			build:   "      repo: https://github.com/example/app.git\n      branch: main\n      tag: v1.0.0\n",
			wantErr: true,
		},
		{
			name:    "commit without ref",
			build:   "      repo: https://github.com/example/app.git\n      commit: deadbeef\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := "name: demo\nresources:\n  app:\n    build:\n" + tt.build
			_, err := stackfile.Load(write(t, "stackfile.yaml", content))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, want error %t", err, tt.wantErr)
			}
		})
	}
}

// The compose converter must emit YAML the hub package accepts.
func TestFromComposeOutputValidates(t *testing.T) {
	compose := write(t, "docker-compose.yaml", `services:
  web:
    image: nginx:latest
    ports:
      - "8080:3000"
    environment:
      APP_ENV: production
  db:
    image: postgres:16
    ports:
      - "5432:5432"
    volumes:
      - db-data:/var/lib/postgresql/data
volumes:
  db-data:
`)

	sf, warnings, err := stackfile.FromCompose(compose, "demo")
	if err != nil {
		t.Fatalf("from compose: %v", err)
	}
	if len(warnings.EnvFiles) != 0 || len(warnings.UnsupportedBindMounts) != 0 {
		t.Fatalf("unexpected compose warnings: %+v", warnings)
	}

	out, err := yaml.Marshal(sf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stackfile.Load(write(t, "stackfile.yaml", string(out))); err != nil {
		t.Fatalf("generated stackfile does not validate: %v\n%s", err, out)
	}
}

func TestFromComposeCollectsEveryEnvFileSyntax(t *testing.T) {
	compose := write(t, "compose.yaml", `services:
  string:
    image: nginx:alpine
    env_file: .env.string
  list:
    image: nginx:alpine
    env_file:
      - .env.base
      - .env.override
  mapping:
    image: nginx:alpine
    env_file:
      path: .env.mapping
      required: false
  long-list:
    image: nginx:alpine
    env_file:
      - path: .env.first
        required: true
      - path: .env.second
        format: raw
`)

	_, warnings, err := stackfile.FromCompose(compose, "demo")
	if err != nil {
		t.Fatalf("from compose: %v", err)
	}
	want := map[string][]string{
		"string":    {".env.string"},
		"list":      {".env.base", ".env.override"},
		"mapping":   {".env.mapping"},
		"long-list": {".env.first", ".env.second"},
	}
	if !reflect.DeepEqual(warnings.EnvFiles, want) {
		t.Fatalf("env files = %#v, want %#v", warnings.EnvFiles, want)
	}
}

func TestFromComposeDoesNotCreateNamedVolumeFromWindowsBindMount(t *testing.T) {
	compose := write(t, "compose.yaml", `services:
  web:
    image: nginx:alpine
    volumes:
      - 'C:\data:/data'
`)

	sf, warnings, err := stackfile.FromCompose(compose, "demo")
	if err != nil {
		t.Fatalf("from compose: %v", err)
	}
	if _, exists := sf.Volumes["C"]; exists {
		t.Fatalf("Windows bind mount became bogus named volume C: %+v", sf.Volumes)
	}
	if mounts := sf.Resources["web"].Volumes; len(mounts) != 0 {
		t.Fatalf("Windows bind mount became bogus resource mount: %+v", mounts)
	}
	if got := warnings.UnsupportedBindMounts["web"]; !reflect.DeepEqual(got, []string{`C:\data`}) {
		t.Fatalf("unsupported bind mount warning = %v, want C:\\data", got)
	}
}
