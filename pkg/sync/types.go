package sync

import (
	"context"

	"github.com/ashishmax31/voyager-cli/pkg/provider"
)

type Syncer interface {
	SetupSyncSession(context.Context, SourceDestintionList, provider.Target) error
	StopSyncSession(context.Context) error
	ForceSync(context.Context) error
	Initialized(context.Context) (bool, error)
	SyncSessionRunning(context.Context) (bool, error)
	SyncSessionRunningFlagPath() string
	Status(context.Context) error
}
