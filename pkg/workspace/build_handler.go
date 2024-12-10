package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/config"
)

func (w *workspaceHandler) Build(ctx context.Context, runtime *config.Runtime) error {
	currentWorkspaceName := runtime.Config().CurrentWorkspace
	if currentWorkspaceName == nil {
		return fmt.Errorf("current workspace not set")
	}

	initialized, err := w.syncHandler.Initialized(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to check sync session status: %w", err)
	}
	if initialized {
		if err := w.Sync(ctx); err != nil {
			return err
		}

		if runtime.Args.IsAllResources() {
			fmt.Println("triggering a new build for all resources...")
		} else {
			fmt.Printf("triggering a new build for '%s' resource...\n", runtime.Args.GetResourceName())
		}

		currentWorkspace, err := w.workspaceService.GetWorkspaceByName(
			ctx,
			*currentWorkspaceName,
		)
		if err != nil {
			return workspaceHandlerErr("failed to get current workspace with name '%s': %w", *currentWorkspaceName, err)
		}

		if runtime.Args.IsAllResources() {
			if err := w.workspaceService.TriggerBuildForAllResources(ctx, currentWorkspace); err != nil {
				return workspaceHandlerErr("failed to trigger build for all resources: %w", err)
			}
			fmt.Println("build triggered successfully")
			return nil
		}
		if err := w.workspaceService.TriggerBuildForResource(ctx, currentWorkspace, runtime.Args.GetResourceName()); err != nil {
			return workspaceHandlerErr("failed to trigger build for resource '%s': %w", runtime.Args.GetResourceName(), err)
		}
		fmt.Printf("build triggered successfully for resource '%s'\n", runtime.Args.GetResourceName())
		return nil
	}
	return fmt.Errorf("sync session not running! Please run voyager sync init")
}
