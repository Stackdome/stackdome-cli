package stackfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Stackdome/stackdome/pkg/models"
	"github.com/stackdome/cli/internal/stackfile"
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

func TestLoadMergesEnvFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "web.env"), []byte("FROM_FILE=yes\nPUBLIC_URL=ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "stackfile.yaml")
	if err := os.WriteFile(path, []byte(singlePortSelfRef+"    env_file: web.env\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sf, err := stackfile.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	env := sf.Resources["web"].Env
	if env["FROM_FILE"] != "yes" {
		t.Fatalf("env_file value not merged: %v", env)
	}
	if env["PUBLIC_URL"] != "{{ self.public_url }}" {
		t.Fatalf("env_file must not override env: %v", env)
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

	sf, envFiles, err := stackfile.FromCompose(compose, "demo")
	if err != nil {
		t.Fatalf("from compose: %v", err)
	}
	if len(envFiles) != 0 {
		t.Fatalf("unexpected env files: %v", envFiles)
	}

	out, err := yaml.Marshal(sf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stackfile.Load(write(t, "stackfile.yaml", string(out))); err != nil {
		t.Fatalf("generated stackfile does not validate: %v\n%s", err, out)
	}
}
