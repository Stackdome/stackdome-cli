package sync

import (
	"context"
)

type Syncer interface {
	SetupSyncSession(ctx context.Context, spec SourceDestintionList, dstSSHhandler DestinationSSHTunnelHandler) error
	Sync(context.Context) error
	Initialized(context.Context) (bool, error)
	// Status(context.Context) (string, error)
}
