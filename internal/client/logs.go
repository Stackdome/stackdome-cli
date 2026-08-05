package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	clierrors "github.com/stackdome/cli/internal/errors"
)

type LogOptions struct {
	Follow bool
	Tail   int32
	Since  string
}

func (c *Client) StreamLogs(ctx context.Context, stackID, resourceName string, opts LogOptions) (io.ReadCloser, error) {
	path := fmt.Sprintf("/api/v1/organizations/%s/projects/%s/stacks/%s", c.orgID, c.projectName, stackID)
	if resourceName != "" {
		path += fmt.Sprintf("/resources/%s", resourceName)
	}
	path += "/logs"

	return c.openLogStream(ctx, path, opts)
}

func (c *Client) openLogStream(ctx context.Context, path string, opts LogOptions) (io.ReadCloser, error) {
	query := "?tail=" + strconv.Itoa(int(opts.Tail))
	if opts.Follow {
		query += "&follow=true"
	}
	if opts.Since != "" {
		query += "&since=" + opts.Since
	}

	url := c.baseURL + path + query

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, clierrors.Wrap(err, "Failed to create log request")
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Accept", "text/event-stream")

	// `-f` follows a log stream indefinitely; reuse the configured transport (so
	// token refresh still applies) but not its 30s whole-request timeout. The
	// context governs the stream's lifetime.
	httpClient := &http.Client{}
	if c.cfg.HTTPClient != nil {
		httpClient.Transport = c.cfg.HTTPClient.Transport
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, clierrors.Wrap(err, "Failed to connect to log stream")
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, clierrors.FromHTTP(resp.StatusCode, "Log streaming failed")
	}

	return resp.Body, nil
}
