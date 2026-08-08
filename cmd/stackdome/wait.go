package main

import (
	"context"
	"time"

	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
)

// defaultWaitTimeout bounds commands that follow asynchronous server work.
// Callers may provide a positive timeout to override it.
const defaultWaitTimeout = 10 * time.Minute

func waitContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultWaitTimeout
	}
	return context.WithTimeout(parent, timeout)
}

// waitCommandError gives the entire wait-and-observe sequence one stable
// cancellation contract. The child context represents the command timeout;
// an interrupted parent remains the user-cancellation contract.
func waitCommandError(parent, wait context.Context, err error) error {
	if parent.Err() != nil {
		return clierrors.ErrUserCanceled
	}
	if wait.Err() == context.DeadlineExceeded {
		return clierrors.New("Timed out waiting for the release to finish.").WithCode("TIMEOUT")
	}
	return err
}
