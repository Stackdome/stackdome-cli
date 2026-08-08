package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	pathpkg "path"
	"strings"
)

// APIResponse is a buffered response from a direct API request.
type APIResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// APIRequest sends an authenticated request to a relative /api/ path through
// the client's refresh-aware transport. Redirects may stay on the configured
// origin, but are rejected before a request can leave it.
func (c *Client) APIRequest(ctx context.Context, method, path string, headers http.Header, body []byte) (APIResponse, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return APIResponse{}, fmt.Errorf("invalid configured server URL")
	}

	target, err := parseAPIPath(path)
	if err != nil {
		return APIResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, method, base.ResolveReference(target).String(), bytes.NewReader(body))
	if err != nil {
		return APIResponse{}, fmt.Errorf("create API request: %w", err)
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return APIResponse{}, err
	}
	defer resp.Body.Close()

	response := APIResponse{StatusCode: resp.StatusCode, Header: resp.Header.Clone()}
	response.Body, err = io.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}
	return response, nil
}

func parseAPIPath(raw string) (*url.URL, error) {
	target, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid API path: %w", err)
	}
	if target.IsAbs() || target.Host != "" || target.Fragment != "" || !strings.HasPrefix(target.Path, "/api/") || !isWithinAPIPath(target.Path) {
		return nil, fmt.Errorf("API path must be a relative path beginning with /api/")
	}
	return target, nil
}

// ValidateAPIPath reports whether raw is a safe direct API path. It is
// exported so the command can reject an unsafe path before constructing a
// client request while retaining the client's defense in depth.
func ValidateAPIPath(raw string) error {
	_, err := parseAPIPath(raw)
	return err
}

func isWithinAPIPath(requestPath string) bool {
	for {
		cleaned := pathpkg.Clean(requestPath)
		if cleaned != "/api" && !strings.HasPrefix(cleaned, "/api/") {
			return false
		}
		decoded, err := url.PathUnescape(requestPath)
		if err != nil || decoded == requestPath {
			return err == nil
		}
		requestPath = decoded
	}
}

func sameOriginRedirectPolicy(base *url.URL) func(*http.Request, []*http.Request) error {
	return func(redirect *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		if base == nil || redirect.URL.User != nil || !sameAPIOrigin(base, redirect.URL) {
			return fmt.Errorf("refusing redirect to a different origin")
		}
		return nil
	}
}

func sameAPIOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && canonicalHostPort(a) == canonicalHostPort(b)
}

func canonicalHostPort(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	return net.JoinHostPort(host, port)
}
