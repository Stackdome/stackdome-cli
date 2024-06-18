package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/mapper"
	"github.com/ashishmax31/voyager-cli/pkg/tools"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

func (w *WorkspaceHandler) Build(ctx context.Context, resourceName string) error {
	initialized, err := w.syncHandler.Initialized(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to check sync session status: %w", err)
	}
	if initialized {
		if err := w.Sync(ctx); err != nil {
			return err
		}
		if resourceName == "all" {
			fmt.Println("triggering a new build for all resources...")
		} else {
			fmt.Printf("triggering a new build for '%s' resource...\n", resourceName)
		}
		desiredWS := mapper.MapVoyagerFileToWorkspaceCR(
			w.userdefinedWorkspace,
			w.session.Config().Username,
			w.session.Config().ProviderConfig.Namespace,
			w.session.Config().Organisation,
			w.session.Config().ProviderConfig.WorkspaceDomain,
		)
		existingWS, present, err := w.getWorkspace(ctx, desiredWS)
		if err != nil {
			return err
		}
		if present {
			desiredWS.ResourceVersion = existingWS.ResourceVersion
			w.copyBuildSourceHash(existingWS, desiredWS)
			if resourceName == "all" {
				w.setBuildHashForResources(desiredWS)
			} else {
				w.setNewBuildSourceHashForResource(desiredWS, resourceName)
			}
			return w.session.UpdateResourceInProvider(ctx, desiredWS)
		}

		// Set the initial build source hash for the workspace resources.
		w.setBuildHashForResources(desiredWS)
		return w.session.CreateResourceInProvider(
			ctx,
			desiredWS,
		)
	}
	return fmt.Errorf("sync session not running! Please run voyager sync init")
}

func (w *WorkspaceHandler) setNewBuildSourceHashForResource(ws *workspacev1alpha1.Workspace, resourceName string) {
	for i := range ws.Spec.Resources {
		currResource := &ws.Spec.Resources[i]
		if currResource.Name == resourceName && currResource.Spec.ApplicationBuildSpec != nil {
			currResource.Spec.ApplicationBuildSpec.BuildSourceHash = tools.GenRandomHash()
		}
	}
}
