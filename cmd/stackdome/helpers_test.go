package main

import (
	"testing"

	clierrors "github.com/stackdome/cli/internal/errors"
)

func TestResolveIDPrefix(t *testing.T) {
	ids := []string{"web-7e81aaaa", "web-7e81bbbb", "api-1234cccc", "api-1234"}

	tests := []struct {
		name     string
		arg      string
		want     string
		wantExit int
	}{
		{name: "exact match wins over prefix", arg: "api-1234", want: "api-1234"},
		{name: "unique prefix", arg: "web-7e81a", want: "web-7e81aaaa"},
		{name: "full id", arg: "web-7e81bbbb", want: "web-7e81bbbb"},
		{name: "ambiguous", arg: "web-7e81", wantExit: clierrors.ExitValidation},
		{name: "no match", arg: "zzz", wantExit: clierrors.ExitNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveIDPrefix("Build", tt.arg, ids)
			if tt.wantExit != 0 {
				cliErr, ok := err.(*clierrors.CLIError)
				if !ok {
					t.Fatalf("want CLIError, got %v", err)
				}
				if cliErr.ExitCode != tt.wantExit {
					t.Fatalf("exit code: got %d want %d", cliErr.ExitCode, tt.wantExit)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestLooksLikeUUID(t *testing.T) {
	cases := map[string]bool{
		"b02262ac-8e6e-45cd-b18e-acb5d3f97ce4": true,
		"B02262AC-8E6E-45CD-B18E-ACB5D3F97CE4": true,
		"hello-stack":                          false,
		"":                                     false,
		"b02262ac8e6e45cdb18eacb5d3f97ce4":     false,
		"b02262ac-8e6e-45cd-b18e-acb5d3f97cez": false,
	}
	for in, want := range cases {
		if got := looksLikeUUID(in); got != want {
			t.Errorf("looksLikeUUID(%q) = %v, want %v", in, got, want)
		}
	}
}
