package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/mapper"
)

func (w *WorkspaceHandler) Deploy(ctx context.Context) error {
	initialized, err := w.syncHandler.Initialized(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to check sync session status: %w", err)
	}
	if initialized {
		if err := w.Sync(ctx); err != nil {
			return err
		}
		fmt.Println("Deploying voyagerfile to provider...")
		desiredWS := mapper.MapVoyagerFileToWorkspaceCR(
			w.userdefinedWorkspace,
			w.session.Config().Username,
			w.session.Config().ProviderConfig.Namespace,
		)
		present, err := w.workspacePresent(ctx, desiredWS)
		if err != nil {
			return err
		}
		if present {
			return w.session.UpdateResourceInProvider(ctx, desiredWS)
		}
		return w.session.CreateResourceInProvider(
			ctx,
			desiredWS,
		)
	}
	return fmt.Errorf("sync session not running! Please run voyager sync init")
}
