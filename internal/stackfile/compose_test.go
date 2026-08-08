package stackfile_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Stackdome/stackdome-cli/internal/stackfile"
)

func TestFromComposeReportsUnsupportedKeysAndUnresolvedEnvironmentDeterministically(t *testing.T) {
	compose := write(t, "compose.yaml", `version: "3.9"
services:
  worker:
    image: busybox:latest
    environment:
      - SET=worker
      - LIST_MISSING
  web:
    image: nginx:alpine
    restart: always
    healthcheck:
      test: [CMD, curl, -f, http://localhost]
    networks: [frontend]
    secrets: [api-key]
    configs: [web-config]
    deploy:
      replicas: 2
    profiles: [production]
    mystery: true
    environment:
      SET: production
      EMPTY: ""
      NUMBER: 42
      MAP_MISSING:
networks:
  frontend: {}
secrets:
  api-key:
    external: true
configs:
  web-config:
    file: ./web.conf
mystery: true
`)

	sf, warnings, err := stackfile.FromCompose(compose, "demo")
	if err != nil {
		t.Fatalf("FromCompose: %v", err)
	}

	if want := []string{"configs", "mystery", "networks", "secrets"}; !reflect.DeepEqual(warnings.UnsupportedTopLevelKeys, want) {
		t.Errorf("top-level warnings = %#v, want %#v", warnings.UnsupportedTopLevelKeys, want)
	}
	if want := []string{"configs", "deploy", "healthcheck", "mystery", "networks", "profiles", "restart", "secrets"}; !reflect.DeepEqual(warnings.UnsupportedServiceKeys["web"], want) {
		t.Errorf("web unsupported keys = %#v, want %#v", warnings.UnsupportedServiceKeys["web"], want)
	}
	if want := map[string][]string{"web": {"MAP_MISSING"}, "worker": {"LIST_MISSING"}}; !reflect.DeepEqual(warnings.UnresolvedEnvironment, want) {
		t.Errorf("unresolved environment = %#v, want %#v", warnings.UnresolvedEnvironment, want)
	}

	webEnv := sf.Resources["web"].Env
	if want := map[string]string{"EMPTY": "", "NUMBER": "42", "SET": "production"}; !reflect.DeepEqual(webEnv, want) {
		t.Errorf("web env = %#v, want %#v", webEnv, want)
	}
	if got := sf.Resources["worker"].Env; !reflect.DeepEqual(got, map[string]string{"SET": "worker"}) {
		t.Errorf("worker env = %#v, want only explicit value", got)
	}
	for name, res := range sf.Resources {
		for key, value := range res.Env {
			if value == "<nil>" || strings.Contains(value, "MAP_MISSING") || strings.Contains(value, "LIST_MISSING") {
				t.Errorf("resource %s invented environment %s=%q", name, key, value)
			}
		}
	}
}

func TestFromComposeReportsNestedUnsupportedOptionsDeterministically(t *testing.T) {
	compose := write(t, "compose.yaml", `services:
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
    environment:
      PLAIN: supported
      COMPLEX:
        nested: value
      LIST: [one, two]
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
`)

	sf, warnings, err := stackfile.FromCompose(compose, "demo")
	if err != nil {
		t.Fatalf("FromCompose: %v", err)
	}

	assertStrings := func(label string, got, want []string) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %#v, want %#v", label, got, want)
		}
	}
	assertStrings("build options", warnings.UnsupportedBuildOptions["web"], []string{"args", "cache_from", "target"})
	assertStrings("depends_on options", warnings.UnsupportedDependsOnOptions["web"], []string{"db.condition", "db.required", "db.restart"})
	assertStrings("top-level volume options", warnings.UnsupportedVolumeOptions["data"], []string{"driver", "driver_opts", "external", "labels", "name"})
	assertStrings("mount options", warnings.UnsupportedVolumeMountOptions["web"], []string{"data:/srv/data:ro"})
	assertStrings("ports", warnings.UnsupportedPorts["web"], []string{"8000-8002:9000-9002", "not-a-port"})
	assertStrings("complex environment", warnings.UnresolvedEnvironment["web"], []string{"COMPLEX", "LIST"})

	web := sf.Resources["web"]
	if web.Build == nil || web.Build.Context != "." || web.Build.Dockerfile != "Containerfile" {
		t.Errorf("supported build fields = %#v, want context and dockerfile preserved", web.Build)
	}
	assertStrings("dependencies", web.DependsOn, []string{"db"})
	if want := map[string]string{"PLAIN": "supported"}; !reflect.DeepEqual(web.Env, want) {
		t.Errorf("environment = %#v, want only supported scalar %#v", web.Env, want)
	}
	if want := []stackfile.VolumeMountDef{{Name: "data", Path: "/srv/data"}, {Name: "cache", Path: "/srv/cache"}}; !reflect.DeepEqual(web.Volumes, want) {
		t.Errorf("volume mounts = %#v, want %#v", web.Volumes, want)
	}
	if want := []stackfile.PortDef{{Name: "http", Port: 80, Protocol: "TCP", Public: true}}; !reflect.DeepEqual(web.Ports, want) {
		t.Errorf("ports = %#v, want only supported port %#v", web.Ports, want)
	}
	if _, ok := sf.Volumes["data"]; !ok {
		t.Error("named volume data was not preserved")
	}
	if _, ok := sf.Volumes["cache"]; !ok {
		t.Error("named volume cache was not preserved")
	}
	if _, ok := warnings.UnsupportedVolumeOptions["cache"]; ok {
		t.Errorf("empty volume definition should not warn: %#v", warnings.UnsupportedVolumeOptions["cache"])
	}
}

func TestFromComposePreservesPortExposureWithoutBroadeningHostIPBindings(t *testing.T) {
	compose := write(t, "compose.yaml", `services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
      - "5432:5432"
      - "127.0.0.1:8081:81"
      - "0.0.0.0:82:82"
      - "0.0.0.0:8084:84"
      - "192.0.2.10:8083:83"
`)

	sf, warnings, err := stackfile.FromCompose(compose, "demo")
	if err != nil {
		t.Fatalf("FromCompose: %v", err)
	}

	publicByPort := make(map[int32]bool)
	for _, port := range sf.Resources["web"].Ports {
		publicByPort[port.Port] = port.Public
	}
	want := map[int32]bool{
		80:   true,
		5432: true,
		81:   false,
		82:   true,
		84:   true,
		83:   false,
	}
	if !reflect.DeepEqual(publicByPort, want) {
		t.Errorf("port exposure = %#v, want %#v", publicByPort, want)
	}
	if want := []string{"0.0.0.0:8084:84", "127.0.0.1:8081:81", "192.0.2.10:8083:83", "8080:80"}; !reflect.DeepEqual(warnings.UnsupportedPortMappings["web"], want) {
		t.Errorf("port mapping warnings = %#v, want %#v", warnings.UnsupportedPortMappings["web"], want)
	}
}

func TestFromComposeMakesTCPExplicitAndRejectsUnsupportedUDP(t *testing.T) {
	compose := write(t, "compose.yaml", `services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
      - "8443:443/tcp"
      - "5353:5353/udp"
  db:
    image: postgres:16
    ports:
      - "5432:5432"
`)

	sf, warnings, err := stackfile.FromCompose(compose, "demo")
	if err != nil {
		t.Fatalf("FromCompose: %v", err)
	}

	webProtocols := make(map[int32]string)
	for _, port := range sf.Resources["web"].Ports {
		webProtocols[port.Port] = port.Protocol
	}
	if want := map[int32]string{80: "TCP", 443: "TCP"}; !reflect.DeepEqual(webProtocols, want) {
		t.Errorf("web protocols = %#v, want explicit Compose TCP %#v", webProtocols, want)
	}
	dbPorts := sf.Resources["db"].Ports
	if len(dbPorts) != 1 || dbPorts[0].Port != 5432 || !dbPorts[0].Public || dbPorts[0].Protocol != "TCP" {
		t.Errorf("db ports = %#v, want public 5432/TCP", dbPorts)
	}
	if want := []string{"5353:5353/udp"}; !reflect.DeepEqual(warnings.UnsupportedPorts["web"], want) {
		t.Errorf("unsupported ports = %#v, want %#v", warnings.UnsupportedPorts["web"], want)
	}
}

func TestFromComposePreservesListCommandBoundariesAndOmitsAmbiguousStrings(t *testing.T) {
	compose := write(t, "compose.yaml", `services:
  string-command:
    image: busybox:latest
    command: /bin/sh -c 'echo "hello world"'
  list-command:
    image: busybox:latest
    command:
      - /bin/sh
      - -c
      - echo "hello world"
  entrypoint-and-command:
    image: busybox:latest
    entrypoint:
      - /bin/sh
      - -c
    command:
      - echo "hello world"
  string-entrypoint:
    image: busybox:latest
    entrypoint: /bin/sh -c
  empty-forms:
    image: busybox:latest
    entrypoint: []
    command: []
`)

	sf, warnings, err := stackfile.FromCompose(compose, "demo")
	if err != nil {
		t.Fatalf("FromCompose: %v", err)
	}

	stringCommand := sf.Resources["string-command"]
	if stringCommand.Command != nil || stringCommand.Args != nil {
		t.Errorf("ambiguous string command = command %#v args %#v, want omitted", stringCommand.Command, stringCommand.Args)
	}
	listCommand := sf.Resources["list-command"]
	if listCommand.Command != nil {
		t.Errorf("command-only list mapped to command %#v, want nil", listCommand.Command)
	}
	if want := []string{"/bin/sh", "-c", `echo "hello world"`}; !reflect.DeepEqual(listCommand.Args, want) {
		t.Errorf("command-only args = %#v, want exact boundaries %#v", listCommand.Args, want)
	}
	combined := sf.Resources["entrypoint-and-command"]
	if want := []string{"/bin/sh", "-c"}; !reflect.DeepEqual(combined.Command, want) {
		t.Errorf("entrypoint command = %#v, want %#v", combined.Command, want)
	}
	if want := []string{`echo "hello world"`}; !reflect.DeepEqual(combined.Args, want) {
		t.Errorf("entrypoint args = %#v, want %#v", combined.Args, want)
	}
	stringEntrypoint := sf.Resources["string-entrypoint"]
	if stringEntrypoint.Command != nil || stringEntrypoint.Args != nil {
		t.Errorf("ambiguous string entrypoint = command %#v args %#v, want omitted", stringEntrypoint.Command, stringEntrypoint.Args)
	}
	if want := []string{"command"}; !reflect.DeepEqual(warnings.UnsupportedCommandForms["string-command"], want) {
		t.Errorf("string command warnings = %#v, want %#v", warnings.UnsupportedCommandForms["string-command"], want)
	}
	if want := []string{"entrypoint"}; !reflect.DeepEqual(warnings.UnsupportedCommandForms["string-entrypoint"], want) {
		t.Errorf("string entrypoint warnings = %#v, want %#v", warnings.UnsupportedCommandForms["string-entrypoint"], want)
	}
	if _, ok := warnings.UnsupportedCommandForms["list-command"]; ok {
		t.Errorf("exact list command unexpectedly warned: %#v", warnings.UnsupportedCommandForms["list-command"])
	}
	if _, ok := warnings.UnsupportedCommandForms["entrypoint-and-command"]; ok {
		t.Errorf("exact list entrypoint/command unexpectedly warned: %#v", warnings.UnsupportedCommandForms["entrypoint-and-command"])
	}
	if want := []string{"command", "entrypoint"}; !reflect.DeepEqual(warnings.UnsupportedCommandForms["empty-forms"], want) {
		t.Errorf("empty command-form warnings = %#v, want %#v", warnings.UnsupportedCommandForms["empty-forms"], want)
	}
}
