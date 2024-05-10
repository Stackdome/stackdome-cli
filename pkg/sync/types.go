package sync

import (
	"context"

	"github.com/ashishmax31/voyager-cli/pkg/provider"
)

type Syncer interface {
	SetupSyncSession(context.Context, SourceDestintionList, provider.Target) error
	Sync(context.Context) error
	Initialized(context.Context) (bool, error)
	// Status(context.Context) (string, error)
}
