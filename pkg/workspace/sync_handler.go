package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/mapper"
	"github.com/ashishmax31/voyager-cli/pkg/provider/k8s"
	"github.com/ashishmax31/voyager-cli/pkg/sync"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

func (w *WorkspaceHandler) StartSyncSession(ctx context.Context) error {
	existingWS := &workspacev1alpha1.WorkspaceStorage{}
	getErr := w.session.GetResourceFromProvider(
		ctx,
		types.NamespacedName{Name: mapper.WorkspaceStorageName(w.session.Config().Username), Namespace: w.session.Config().ProviderConfig.Namespace},
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
		logrus.Info("already initialized")
		return nil
	}
	logrus.Debugf("user workspace volumes: %+v, resources to sync: %+v \n", w.userdefinedWorkspace.Volumes, resourcesToBeSynced)
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
	logrus.Debugf("synclist : %+v \n", syncList)

	// Blocking.
	if err := w.syncHandler.SetupSyncSession(
		ctx,
		syncList,
		k8s.NewServiceTarget(existingWS.Status.ServiceName),
	); err != nil {
		return workspaceHandlerErr("sync session failed: %w", err)
	}
	// Success
	return nil
}

func (w *WorkspaceHandler) SyncStatus(ctx context.Context) error {
	return w.syncHandler.Status(ctx)
}

func (w *WorkspaceHandler) StopSyncSession(ctx context.Context) error {
	return w.syncHandler.StopSyncSession(ctx)
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

func (w *WorkspaceHandler) markAsSynced(ctx context.Context) error {
	existingWS := &workspacev1alpha1.WorkspaceStorage{}
	getErr := w.session.GetResourceFromProvider(
		ctx,
		types.NamespacedName{Name: mapper.WorkspaceStorageName(w.session.Config().Username), Namespace: w.session.Config().ProviderConfig.Namespace},
		existingWS,
	)
	if getErr != nil {
		return getErr
	}
	if existingWS.HasSyncRequiredStorageResources() {
		volumes, err := w.session.GetWorkspaceVolumesFromProvider(ctx, existingWS)
		if err != nil {
			return fmt.Errorf("workspace volumes fetch error: %w", err)
		}
		logrus.Debug("marking as synced")
		for i := range volumes {
			volume := &volumes[i]
			if volume.Spec.NeedsSyncBeforeUse {
				volume.MarkAsSynced()
				if err := w.session.UpdateResourceInProvider(ctx, volume); err != nil {
					return fmt.Errorf("failed to mark volume '%s' as synced: %w", volume.Name, err)
				}
			}
		}
	}
	return nil
}
