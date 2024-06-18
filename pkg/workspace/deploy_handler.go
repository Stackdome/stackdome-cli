package workspace

import (
	"context"
	"fmt"

	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"

	"github.com/ashishmax31/voyager-cli/pkg/mapper"
	"github.com/ashishmax31/voyager-cli/pkg/tools"
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
			w.session.Config().Organisation,
			w.session.Config().ProviderConfig.WorkspaceDomain,
		)
		existingWS, present, err := w.getWorkspace(ctx, desiredWS)
		if err != nil {
			return err
		}
		if present {
			// Set generation
			desiredWS.ResourceVersion = existingWS.ResourceVersion
			w.copyBuildSourceHash(existingWS, desiredWS)
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

func (w *WorkspaceHandler) copyBuildSourceHash(existingWS, desiredWS *workspacev1alpha1.Workspace) {

	// Copy over build hash of existing resources which has application build spec.
	currentBuildHashMap := make(map[string]string)

	for i := range existingWS.Spec.Resources {
		currResource := &existingWS.Spec.Resources[i]
		if currResource.Spec.ApplicationBuildSpec != nil {
			currentBuildHashMap[currResource.Name] = currResource.Spec.ApplicationBuildSpec.BuildSourceHash
		}
	}
	for i := range desiredWS.Spec.Resources {
		currResource := &desiredWS.Spec.Resources[i]
		if currResource.Spec.ApplicationBuildSpec != nil {
			if _, found := currentBuildHashMap[currResource.Name]; found {
				currResource.Spec.ApplicationBuildSpec.BuildSourceHash = currentBuildHashMap[currResource.Name]
			} else {
				currResource.Spec.ApplicationBuildSpec.BuildSourceHash = tools.GenRandomHash()
			}
		}
	}
}

func (w *WorkspaceHandler) setBuildHashForResources(desiredWS *workspacev1alpha1.Workspace) {
	for i := range desiredWS.Spec.Resources {
		currResource := &desiredWS.Spec.Resources[i]
		if currResource.Spec.ApplicationBuildSpec != nil {
			currResource.Spec.ApplicationBuildSpec.BuildSourceHash = tools.GenRandomHash()
		}
	}
}
