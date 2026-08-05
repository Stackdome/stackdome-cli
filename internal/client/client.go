package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	serverapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	clierrors "github.com/stackdome/cli/internal/errors"
)

const defaultTimeout = 30 * time.Second

type Client struct {
	apiClient      *serverapi.APIClient
	cfg            *serverapi.Configuration
	accessToken    string
	refreshToken   string
	orgID          string
	projectName    string
	baseURL        string
	onTokenRefresh func(accessToken, refreshToken string) error
}

type Option func(*Client)

func WithInsecure(insecure bool) Option {
	return func(c *Client) {
		if insecure {
			c.cfg.HTTPClient = &http.Client{
				Timeout: defaultTimeout,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			}
		}
	}
}

func WithTokens(accessToken, refreshToken string) Option {
	return func(c *Client) {
		c.accessToken = accessToken
		c.refreshToken = refreshToken
	}
}

func WithOrgAndProject(orgID, projectName string) Option {
	return func(c *Client) {
		c.orgID = orgID
		c.projectName = projectName
	}
}

func WithTokenRefreshCallback(fn func(accessToken, refreshToken string) error) Option {
	return func(c *Client) {
		c.onTokenRefresh = fn
	}
}

func New(baseURL string, opts ...Option) *Client {
	cfg := serverapi.NewConfiguration()
	cfg.Servers = serverapi.ServerConfigurations{
		{URL: baseURL},
	}
	cfg.UserAgent = "stackdome-cli"
	cfg.HTTPClient = &http.Client{
		Timeout: defaultTimeout,
	}

	c := &Client{
		cfg:     cfg,
		baseURL: baseURL,
	}

	for _, opt := range opts {
		opt(c)
	}

	base := cfg.HTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	cfg.HTTPClient.Transport = &refreshTransport{base: base, client: c}

	c.apiClient = serverapi.NewAPIClient(cfg)
	c.applyAuth()
	return c
}

// noRetryKey marks the refresh call itself, so a 401 on it cannot recurse.
type noRetryKey struct{}

// refreshTransport turns a 401 into one refresh + one retry. The server rotates
// the refresh token on every use, so the new pair is persisted immediately.
type refreshTransport struct {
	base   http.RoundTripper
	client *Client
	mu     sync.Mutex
}

func (t *refreshTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}
	if req.Context().Value(noRetryKey{}) != nil || !t.client.canRefresh() {
		return resp, nil
	}

	// Buffer the body up front: a request we cannot replay is not retryable.
	var body io.ReadCloser
	if req.Body != nil {
		if req.GetBody == nil {
			return resp, nil
		}
		if body, err = req.GetBody(); err != nil {
			return resp, nil
		}
	}

	stale := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	t.mu.Lock()
	// Skip if a concurrent request already rotated the pair — reusing a spent
	// refresh token would revoke the session.
	if t.client.accessToken == stale {
		err = t.client.TryRefreshToken(context.WithValue(req.Context(), noRetryKey{}, true))
	}
	token := t.client.accessToken
	t.mu.Unlock()
	if err != nil {
		if body != nil {
			body.Close()
		}
		return resp, nil
	}

	resp.Body.Close()
	retry := req.Clone(req.Context())
	retry.Body = body
	retry.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(retry)
}

// canRefresh reports whether a refresh is even meaningful: env tokens and
// `sdm_` personal access tokens come without a refresh pair.
func (c *Client) canRefresh() bool {
	return c.refreshToken != "" && !strings.HasPrefix(c.accessToken, "sdm_")
}

func (c *Client) API() *serverapi.DefaultApiService {
	return c.apiClient.DefaultApi
}

func (c *Client) applyAuth() {
	if c.accessToken != "" {
		c.cfg.DefaultHeader["Authorization"] = "Bearer " + c.accessToken
	}
}

func (c *Client) SetTokens(accessToken, refreshToken string) {
	c.accessToken = accessToken
	c.refreshToken = refreshToken
	c.applyAuth()
}

func (c *Client) SetOrgAndProject(orgID, projectName string) {
	c.orgID = orgID
	c.projectName = projectName
}

func (c *Client) OrgID() string       { return c.orgID }
func (c *Client) ProjectName() string { return c.projectName }

func (c *Client) ProjectPath() string {
	return fmt.Sprintf("/organizations/%s/projects/%s", c.orgID, c.projectName)
}

func (c *Client) TryRefreshToken(ctx context.Context) error {
	req := c.apiClient.DefaultApi.ApiV1AuthRefreshPost(ctx)
	req = req.RefreshTokenRequest(serverapi.RefreshTokenRequest{
		RefreshToken: c.refreshToken,
	})

	resp, _, err := req.Execute()
	if err != nil {
		return clierrors.AuthError("Session expired. Run `stackdome login` to re-authenticate.")
	}

	c.accessToken = resp.GetToken()
	c.refreshToken = resp.GetRefreshToken()
	c.applyAuth()

	if c.onTokenRefresh != nil {
		return c.onTokenRefresh(c.accessToken, c.refreshToken)
	}
	return nil
}

func WrapError(httpResp *http.Response, err error, message string) error {
	if httpResp != nil {
		reason := extractAPIReason(err)
		if reason != "" {
			return clierrors.FromHTTP(httpResp.StatusCode, reason)
		}
		return clierrors.FromHTTP(httpResp.StatusCode, err.Error())
	}
	if isTimeoutError(err) {
		return clierrors.Wrapf(err, "%s: request timed out", message)
	}
	return clierrors.Wrapf(err, "%s", message)
}

type bodyer interface {
	Body() []byte
}

func extractAPIReason(err error) string {
	if err == nil {
		return ""
	}

	var sources [][]byte
	if b, ok := err.(bodyer); ok {
		sources = append(sources, b.Body())
	}
	sources = append(sources, []byte(err.Error()))

	var apiErr struct {
		Reason string `json:"reason"`
	}
	for _, src := range sources {
		if json.Unmarshal(src, &apiErr) == nil && apiErr.Reason != "" {
			return apiErr.Reason
		}
	}
	return ""
}

func isTimeoutError(err error) bool {
	if urlErr, ok := err.(*url.Error); ok && urlErr.Timeout() {
		return true
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return err == context.DeadlineExceeded
}
