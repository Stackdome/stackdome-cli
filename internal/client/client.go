package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	serverapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	clierrors "github.com/stackdome/cli/internal/errors"
)

const defaultTimeout = 30 * time.Second

type Client struct {
	apiClient      *serverapi.APIClient
	cfg            *serverapi.Configuration
	accessToken    string
	refreshToken   string
	orgID          string
	teamName       string
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

func WithOrgAndTeam(orgID, teamName string) Option {
	return func(c *Client) {
		c.orgID = orgID
		c.teamName = teamName
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

	c.apiClient = serverapi.NewAPIClient(cfg)
	c.applyAuth()
	return c
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

func (c *Client) SetOrgAndTeam(orgID, teamName string) {
	c.orgID = orgID
	c.teamName = teamName
}

func (c *Client) OrgID() string    { return c.orgID }
func (c *Client) TeamName() string { return c.teamName }

func (c *Client) TeamPath() string {
	return fmt.Sprintf("/organizations/%s/teams/%s", c.orgID, c.teamName)
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
