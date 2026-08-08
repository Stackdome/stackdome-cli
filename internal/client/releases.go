package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
)

// reconnectBackoff is the pause before retrying a *failed* reconnect. A clean
// drop reconnects immediately.
const reconnectBackoff = time.Second

// maxReconnectAttempts bounds consecutive failed reconnects; any delivered
// event resets the counter.
const maxReconnectAttempts = 5

func (c *Client) ListReleases(ctx context.Context, stackID string) ([]openapi.StackRelease, error) {
	resp, httpResp, err := c.apiClient.ReleasesApi.
		ListReleases(ctx, c.orgID, c.projectName, stackID).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list releases")
	}
	return resp.Items, nil
}

// CreateRelease starts a new release from the stack's current document. An
// apply only stores the document — this is what actually rolls it out.
func (c *Client) CreateRelease(ctx context.Context, stackID string) (*openapi.StackRelease, error) {
	resp, httpResp, err := c.apiClient.ReleasesApi.
		CreateRelease(ctx, c.orgID, c.projectName, stackID).
		CreateReleaseRequest(openapi.CreateReleaseRequest{}).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to create release")
	}
	return resp, nil
}

// RollbackRelease creates a new release using the document recorded by a
// previous release. The API intentionally models rollback as release creation
// with from_release_id rather than a separate endpoint.
func (c *Client) RollbackRelease(ctx context.Context, stackID, fromReleaseID string) (*openapi.StackRelease, error) {
	resp, httpResp, err := c.apiClient.ReleasesApi.
		CreateRelease(ctx, c.orgID, c.projectName, stackID).
		CreateReleaseRequest(openapi.CreateReleaseRequest{FromReleaseId: openapi.PtrString(fromReleaseID)}).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to roll back release")
	}
	return resp, nil
}

func (c *Client) GetRelease(ctx context.Context, stackID, releaseID string) (*openapi.StackReleaseDetail, error) {
	resp, httpResp, err := c.apiClient.ReleasesApi.
		GetRelease(ctx, c.orgID, c.projectName, stackID, releaseID).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get release")
	}
	return resp, nil
}

func (c *Client) CancelRelease(ctx context.Context, stackID, releaseID string) error {
	httpResp, err := c.apiClient.ReleasesApi.
		CancelRelease(ctx, c.orgID, c.projectName, stackID, releaseID).Execute()
	if err != nil {
		return WrapError(httpResp, err, "Failed to cancel release")
	}
	return nil
}

func (c *Client) ListReleaseEvents(ctx context.Context, stackID, releaseID string, afterSequence int32) ([]openapi.ReleaseEvent, error) {
	resp, httpResp, err := c.apiClient.ReleasesApi.
		ListReleaseEvents(ctx, c.orgID, c.projectName, stackID, releaseID).
		AfterSequence(afterSequence).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list release events")
	}
	return resp.Items, nil
}

// errStreamEnd marks the server's terminal `event: end` — a clean close, not a
// dropped connection, so the stream must not reconnect.
var errStreamEnd = errors.New("stream ended")

// StreamReleaseEvents follows a release's SSE event stream, resuming from the
// last sequence seen when the connection drops. The channel closes when the
// server signals `event: end`, the context is cancelled, or reconnection gives
// up. Events are delivered exactly once.
func (c *Client) StreamReleaseEvents(ctx context.Context, stackID, releaseID string, afterSequence int32) (<-chan SSEEvent, error) {
	body, err := c.openReleaseEventStream(ctx, stackID, releaseID, afterSequence)
	if err != nil {
		return nil, err
	}

	out := make(chan SSEEvent)
	go func() {
		defer close(out)

		last := afterSequence
		attempts := 0

		for {
			progressed := false
			err := ParseSSEStream(body, func(e SSEEvent) error {
				if e.Event == "end" {
					return errStreamEnd
				}
				if seq, ok := eventSequence(e.Data); ok {
					if seq <= last {
						return nil // already delivered before the drop
					}
					last = seq
				}
				select {
				case out <- e:
					progressed = true
				case <-ctx.Done():
					return ctx.Err()
				}
				if e.Event == "error" {
					return errStreamEnd
				}
				return nil
			})
			body.Close()

			if err != nil || ctx.Err() != nil {
				return // clean end, error event, or cancellation
			}

			// EOF without an end event: the connection dropped mid-release.
			// Only a round that delivered nothing counts against the budget,
			// so a healthy but flaky stream reconnects indefinitely while a
			// server that instantly EOFs cannot spin us hot.
			if progressed {
				attempts = 0
			} else {
				attempts++
			}
			if attempts > maxReconnectAttempts {
				select {
				case out <- SSEEvent{Event: "error", Data: "lost connection to release event stream"}:
				case <-ctx.Done():
				}
				return
			}
			if attempts > 1 {
				select {
				case <-time.After(reconnectBackoff):
				case <-ctx.Done():
					return
				}
			}
			body, err = c.openReleaseEventStream(ctx, stackID, releaseID, last)
			if err != nil {
				body = io.NopCloser(emptyReader{})
			}
		}
	}()

	return out, nil
}

type emptyReader struct{}

func (emptyReader) Read([]byte) (int, error) { return 0, io.EOF }

func (c *Client) openReleaseEventStream(ctx context.Context, stackID, releaseID string, afterSequence int32) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/projects/%s/stacks/%s/releases/%s/events/stream?after_sequence=%d",
		c.baseURL, c.orgID, c.projectName, stackID, releaseID, afterSequence)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, clierrors.Wrap(err, "Failed to create release event request")
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Accept", "text/event-stream")

	// A release can be quiet for minutes; reuse the configured transport (so
	// token refresh still applies) but not its 30s whole-request timeout.
	httpClient := c.streamHTTPClient()

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, clierrors.Wrap(err, "Failed to connect to release event stream")
	}
	if resp.StatusCode != http.StatusOK {
		streamErr := wrapHTTPResponseError(resp, "Release event streaming failed")
		resp.Body.Close()
		return nil, streamErr
	}
	return resp.Body, nil
}

// eventSequence pulls the monotonic sequence out of an event payload so a
// reconnect can resume from it.
func eventSequence(data string) (int32, bool) {
	var payload struct {
		Sequence *int32 `json:"sequence"`
	}
	if json.Unmarshal([]byte(data), &payload) != nil || payload.Sequence == nil {
		return 0, false
	}
	return *payload.Sequence, true
}
