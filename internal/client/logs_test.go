package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
)

func TestStreamBuildLogsPreservesServerReason(t *testing.T) {
	const reason = "no logs available for build build-1: build build-1 no longer exists in the cluster"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"reason":"` + reason + `"}`))
	}))
	defer ts.Close()

	c := New(ts.URL, WithTokens("sdm_test", ""), WithOrgAndProject("org-1", "proj-1"))

	_, err := c.StreamBuildLogs(context.Background(), "stack-1", "build-1", LogOptions{Tail: 200})
	if err == nil {
		t.Fatal("StreamBuildLogs() error = nil, want not found error")
	}
	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("error = %T (%v), want CLIError", err, err)
	}
	if !strings.Contains(cliErr.Detail, reason) {
		t.Fatalf("error detail = %q, want server reason %q", cliErr.Detail, reason)
	}
	if got := clierrors.UserMessage(err); strings.Contains(got, reason) {
		t.Fatalf("user message exposes internal server reason: %q", got)
	}
}
