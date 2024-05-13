package provider

import (
	"context"
)

type Provider interface {
	StorageSSHhandler
	Execute(ctx context.Context, target Target, cmd []string, interactive bool) error
	StreamLogs(ctx context.Context, targets []Target, options LogOptions) error
}

type Target interface {
	TargetName() string
	TargetType() string
}

type LogOptions struct {
	Follow    bool
	TailLines int64
}

type StorageSSHhandler interface {
	SetupSSHTunnel(ctx context.Context, localPort int, target Target) (chan struct{}, error)
	SSHUser() string
}
