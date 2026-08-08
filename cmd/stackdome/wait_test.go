package main

import (
	"context"
	"testing"
	"time"
)

func TestWaitContextUsesDefaultForNonPositiveTimeout(t *testing.T) {
	before := time.Now()
	ctx, cancel := waitContext(context.Background(), 0)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("wait context has no deadline")
	}
	if delta := deadline.Sub(before); delta < defaultWaitTimeout-time.Second || delta > defaultWaitTimeout+time.Second {
		t.Errorf("deadline offset = %s, want about %s", delta, defaultWaitTimeout)
	}
}

func TestWaitContextHonorsExplicitTimeout(t *testing.T) {
	before := time.Now()
	ctx, cancel := waitContext(context.Background(), 25*time.Millisecond)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("wait context has no deadline")
	}
	if delta := deadline.Sub(before); delta < 20*time.Millisecond || delta > 100*time.Millisecond {
		t.Errorf("deadline offset = %s, want about 25ms", delta)
	}
}
