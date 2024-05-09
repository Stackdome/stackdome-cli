package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/mapper"
	"github.com/ashishmax31/voyager-cli/pkg/sync"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

func (w *WorkspaceHandler) StartSyncSession(ctx context.Context) error {
	existingWS := &workspacev1alpha1.WorkspaceStorage{}
	getErr := w.session.GetResourceFromProvider(
		ctx,
		types.NamespacedName{Name: workspacev1alpha1.WorkspaceStorageName(w.session.Config().Username), Namespace: w.session.Config().ProviderConfig.Namespace},
		existingWS,
	)
	if getErr != nil {
		return getErr
	}

	resourcesToBeSynced := []workspacev1alpha1.ResourceStorageSpec{}
	for _, spec := range existingWS.Spec.ResourceStorageSpecs {
		if !spec.DontAllowSync {
			resourcesToBeSynced = append(resourcesToBeSynced, spec)
		}
	}
	// Nothing to do.
	if len(resourcesToBeSynced) == 0 {
		return nil
	}

	initialized, err := w.syncHandler.Initialized(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to check sync status: %w", err)
	}
	if initialized {
		fmt.Println("already initialized")
		return nil
	}
	fmt.Printf("user: %+v, resources to sync: %+v \n", w.userdefinedWorkspace, resourcesToBeSynced)
	syncList := make(sync.SourceDestintionList, 0)
	for volumeName, volumeSpec := range w.userdefinedWorkspace.Volumes {
		if volumeSpec.Source != nil && volumeSpec.Source.LocalDir != nil {
			current := sync.SourceDestintionPair{
				Source:      volumeSpec.Source.LocalDir.Path,
				Destination: existingWS.VolumeInfo(volumeName).Subpath,
			}
			syncList = append(syncList, current)
		}
	}
	fmt.Printf("synclist : %+v \n", syncList)

	// Blocking.
	if err := w.syncHandler.SetupSyncSession(
		ctx,
		syncList,
		w.provider.CreateStorageResourceSSHTunnel(existingWS.Status.ServiceName)); err != nil {
		return workspaceHandlerErr("sync session failed: %w", err)
	}
	// Success
	return nil
}

func (w *WorkspaceHandler) Sync(ctx context.Context) error {
	initialized, err := w.syncHandler.Initialized(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to check sync session status: %w", err)
	}
	if initialized {
		if err := w.syncHandler.Sync(ctx); err != nil {
			return err
		}
		return w.markAsSynced(ctx)
	}
	return fmt.Errorf("sync session not running! Please run voyager sync init")
}

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

func (w *WorkspaceHandler) workspacePresent(ctx context.Context, ws *workspacev1alpha1.Workspace) (bool, error) {
	existing := &workspacev1alpha1.Workspace{}
	if err := w.session.GetResourceFromProvider(ctx,
		types.NamespacedName{Name: ws.Name, Namespace: ws.Namespace},
		existing,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	ws.ResourceVersion = existing.ResourceVersion
	return true, nil
}

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

func (w *WorkspaceHandler) markAsSynced(ctx context.Context) error {
	existingWS := &workspacev1alpha1.WorkspaceStorage{}
	getErr := w.session.GetResourceFromProvider(
		ctx,
		types.NamespacedName{Name: workspacev1alpha1.WorkspaceStorageName(w.session.Config().Username), Namespace: w.session.Config().ProviderConfig.Namespace},
		existingWS,
	)
	if getErr != nil {
		return getErr
	}
	if existingWS.HasSyncRequiredStorageResources() {
		existingWS.MarkAsSynced()
		fmt.Println("marking as synced")
		for _, spec := range existingWS.Spec.ResourceStorageSpecs {
			fmt.Printf("resource needssync: %+v \n", spec.NeedsSync)
		}
		return w.session.UpdateResourceInProvider(ctx, existingWS)
	}
	return nil
}
