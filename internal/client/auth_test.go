package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Deployed servers answer GET /users/current/projects with an empty list even
// when the user has a default project, so ResolveDefaultProject falls back to
// the organisation's project list.
func TestResolveDefaultProjectFallsBackToOrgProjects(t *testing.T) {
	var paths []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/users/current/projects":
			fmt.Fprint(w, `{"items":[],"total":0}`)
		case "/api/v1/organizations/org-1/projects":
			fmt.Fprint(w, `{"items":[{"name":"other"},{"name":"the-default","default_project":true}],"total":2}`)
		default:
			t.Errorf("unexpected request to %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := New(ts.URL, WithTokens("access", ""))

	name, err := c.ResolveDefaultProject(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("ResolveDefaultProject: %v", err)
	}
	if name != "the-default" {
		t.Errorf("project = %q, want the-default", name)
	}
	if len(paths) != 2 || paths[1] != "/api/v1/organizations/org-1/projects" {
		t.Errorf("requests = %v, want the org projects endpoint consulted second", paths)
	}
}

// The per-user endpoint is still the primary source: when it answers, the org
// endpoint must not be touched.
func TestResolveDefaultProjectPrefersCurrentUserProjects(t *testing.T) {
	var paths []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"name":"mine"}],"total":1}`)
	}))
	defer ts.Close()

	c := New(ts.URL, WithTokens("access", ""))

	name, err := c.ResolveDefaultProject(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("ResolveDefaultProject: %v", err)
	}
	if name != "mine" {
		t.Errorf("project = %q, want mine", name)
	}
	if len(paths) != 1 {
		t.Errorf("requests = %v, want only the per-user endpoint", paths)
	}
}
