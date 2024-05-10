package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/mapper"
)

func (w *WorkspaceHandler) Build(ctx context.Context, resourceName string) error {
	initialized, err := w.syncHandler.Initialized(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to check sync session status: %w", err)
	}
	if initialized {
		if err := w.syncHandler.Sync(ctx); err != nil {
			return err
		}
		fmt.Printf("Triggering a new build for %s resource/s...\n", resourceName)
		return w.session.UpsertResourceInProvider(
			ctx,
			mapper.MapVoyagerFileToWorkspaceCR(
				w.userdefinedWorkspace,
				w.session.Config().Username,
				w.session.Config().ProviderConfig.Namespace,
			))
	}
	return fmt.Errorf("sync session not running! Please run voyager sync init")
}
