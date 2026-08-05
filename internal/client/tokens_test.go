package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The create call must send name/scopes/expiry as the API expects and hand back
// the raw token verbatim — it is unrecoverable after this response.
func TestCreateAPIToken(t *testing.T) {
	var gotBody map[string]any
	var gotPath, gotMethod string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"sdm_deadbeef","id":"tok-1","name":"ci","token_prefix":"sdm_dead"}`))
	}))
	defer ts.Close()

	expires := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	c := New(ts.URL, WithTokens("access", ""))
	resp, err := c.CreateAPIToken(context.Background(), "ci", []string{"stacks:*", "secrets:read"}, nil, &expires)
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	if gotMethod != http.MethodPost || gotPath != "/api/v1/api-tokens" {
		t.Errorf("request = %s %s, want POST /api/v1/api-tokens", gotMethod, gotPath)
	}
	if gotBody["name"] != "ci" {
		t.Errorf("name = %v, want ci", gotBody["name"])
	}
	if scopes, _ := gotBody["scopes"].([]any); len(scopes) != 2 || scopes[0] != "stacks:*" {
		t.Errorf("scopes = %v, want [stacks:* secrets:read]", gotBody["scopes"])
	}
	if gotBody["expires_at"] == nil {
		t.Error("expires_at missing from request body")
	}
	if _, ok := gotBody["resource_ids"]; ok {
		t.Errorf("resource_ids sent despite being empty: %v", gotBody["resource_ids"])
	}
	if resp.GetToken() != "sdm_deadbeef" || resp.GetTokenPrefix() != "sdm_dead" {
		t.Errorf("response = %+v, want raw token and prefix preserved", resp)
	}
}

func TestListAPITokensAndErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/api-tokens":
			_, _ = w.Write([]byte(`{"items":[{"id":"tok-1","name":"ci","token_prefix":"sdm_dead","scopes":["stacks:*"]}]}`))
		case "/api/v1/api-tokens/scopes":
			_, _ = w.Write([]byte(`{"full_access_scope":"*:*","items":[{"resource":"stacks","actions":["read","write"]}]}`))
		default: // /api/v1/api-tokens/{id} DELETE
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"reason":"token not found"}`))
		}
	}))
	defer ts.Close()

	c := New(ts.URL, WithTokens("access", ""))
	ctx := context.Background()

	tokens, err := c.ListAPITokens(ctx)
	if err != nil {
		t.Fatalf("ListAPITokens: %v", err)
	}
	if len(tokens) != 1 || tokens[0].GetTokenPrefix() != "sdm_dead" {
		t.Errorf("tokens = %+v, want one token with prefix sdm_dead", tokens)
	}

	scopes, err := c.ListTokenScopes(ctx)
	if err != nil {
		t.Fatalf("ListTokenScopes: %v", err)
	}
	if scopes.GetFullAccessScope() != "*:*" || len(scopes.Items) != 1 {
		t.Errorf("scopes = %+v, want full access *:* and one resource", scopes)
	}

	if err := c.DeleteAPIToken(ctx, "missing"); err == nil {
		t.Fatal("DeleteAPIToken on 404 = nil, want error")
	}
}
