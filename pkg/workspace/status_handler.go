package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
	"github.com/ashishmax31/voyager-cli/pkg/mapper"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

func (w *WorkspaceHandler) Status(ctx context.Context, resourceName string) (*userworkspace.WorkspaceStatus, error) {
	desiredWS := mapper.MapVoyagerFileToWorkspaceCR(
		w.userdefinedWorkspace,
		w.session.Config().Username,
		w.session.Config().ProviderConfig.Namespace,
	)
	desiredWStorage := mapper.MapVoyagerFileToWorkspaceStorage(
		w.userdefinedWorkspace,
		w.session.Config().Username,
		w.session.Config().ProviderConfig.Namespace,
	)
	existingWS, WSpresent, WsErr := w.getWorkspace(ctx, desiredWS)
	existingWStorage, WstoragePresent, WstorageErr := w.getWorkspaceStorage(ctx, &desiredWStorage)

	switch {
	case WstorageErr != nil:
		return nil, WstorageErr
	case WsErr != nil:
		return nil, WsErr
	case !WSpresent:
		return nil, fmt.Errorf("workspace not yet deployed. Please run voyager deploy first.")
	case !WstoragePresent:
		return nil, fmt.Errorf("workspace storage not yet provisioned. Please run voyager init first.")
	case resourceName == "all":
		return w.workspaceStatus(ctx, existingWS, existingWStorage)
	default:
		return w.workspaceResourceStatus(ctx, existingWS, resourceName)
	}

}

func (w *WorkspaceHandler) workspaceStatus(
	ctx context.Context,
	existingWS *workspacev1alpha1.Workspace,
	existingWStorage *workspacev1alpha1.WorkspaceStorage) (*userworkspace.WorkspaceStatus, error) {
	res := &userworkspace.WorkspaceStatus{}
	res.WorkspaceName = existingWS.Name
	res.WorkspaceAvailablityStatus = w.getWorkspaceAvailabilityStatus(existingWS)
	resourceStatuses, err := w.getResourceStatuses(ctx, existingWS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workspace resources from provider: %w", err)
	}
	res.ResourceStatuses = resourceStatuses
	res.VolumeStatuses = w.getVolumeStatuses(ctx, existingWStorage)
	return res, nil
}

func (w *WorkspaceHandler) workspaceResourceStatus(
	ctx context.Context,
	existingWS *workspacev1alpha1.Workspace,
	resourceName string) (*userworkspace.WorkspaceStatus, error) {

	res := &userworkspace.WorkspaceStatus{}
	referencedResource := &workspacev1alpha1.WorkspaceResource{}
	if err := w.session.GetResourceFromProvider(
		ctx,
		types.NamespacedName{Name: resourceName, Namespace: existingWS.Namespace},
		referencedResource); err != nil {
		return nil, err
	}
	res.ResourceStatuses = []userworkspace.ResourceStatus{getResourceStatus(referencedResource, resourceName)}
	return res, nil
}

func (w *WorkspaceHandler) getVolumeStatuses(
	ctx context.Context, existingWStorage *workspacev1alpha1.WorkspaceStorage) []userworkspace.VolumeStatus {
	res := make([]userworkspace.VolumeStatus, 0)
	for _, volumeStatus := range existingWStorage.Status.WorkspaceVolumeStatus {
		curr := userworkspace.VolumeStatus{
			VolumeName: volumeStatus.VolumeName,
			Available:  volumeStatus.Available,
		}
		userVolumeSpec := w.userdefinedWorkspace.Volumes[volumeStatus.VolumeName]
		// Populate LocalPath for volumes which have a local path source.
		if userVolumeSpec.Source != nil && userVolumeSpec.Source.LocalDir != nil {
			curr.LocalPath = ptr.To(userVolumeSpec.Source.LocalDir.Path)
		}
		if volumeStatus.LastSyncedAt != nil {
			curr.LastSyncedAt = ptr.To(volumeStatus.LastSyncedAt.String())
		}
		res = append(res, curr)
	}
	return res
}

func (w *WorkspaceHandler) getWorkspaceAvailabilityStatus(existingWS *workspacev1alpha1.Workspace) userworkspace.WorkspaceAvailablityStatus {
	res := userworkspace.WorkspaceAvailablityStatus{}
	workspaceAvailableCond := meta.FindStatusCondition(existingWS.Status.Conditions, string(workspacev1alpha1.WorkspaceAvailable))
	switch {
	case workspaceAvailableCond == nil:
		res.Available = false
	//TODO: Look at the observed generation and generation?
	case workspaceAvailableCond.Status == metav1.ConditionTrue:
		res.Available = true
		res.Message = workspaceAvailableCond.Message
		res.Reason = workspaceAvailableCond.Reason
	default:
		res.Available = false
		res.Message = workspaceAvailableCond.Message
		res.Reason = workspaceAvailableCond.Reason
	}
	return res
}

func (w *WorkspaceHandler) getResourceStatuses(ctx context.Context, existingWS *workspacev1alpha1.Workspace) ([]userworkspace.ResourceStatus, error) {
	resources := make(map[string]*workspacev1alpha1.WorkspaceResource, 0)
	for _, resourceSpec := range existingWS.Spec.Resources {
		currResource := &workspacev1alpha1.WorkspaceResource{}
		if err := w.session.GetResourceFromProvider(
			ctx,
			types.NamespacedName{Name: resourceSpec.Name, Namespace: existingWS.Namespace},
			currResource); err != nil {
			return nil, err
		}
		resources[resourceSpec.Name] = currResource
	}

	res := []userworkspace.ResourceStatus{}
	for resourceName, resource := range resources {
		res = append(res, getResourceStatus(resource, resourceName))
	}
	return res, nil
}

func getResourceStatus(in *workspacev1alpha1.WorkspaceResource, resourceName string) userworkspace.ResourceStatus {
	availableCond := meta.FindStatusCondition(in.Status.Conditions, string(workspacev1alpha1.WorkspaceResourceStatusAvailable))
	curr := userworkspace.ResourceStatus{
		ResourceName: resourceName,
		Addresses:    mapExternalAddressesToAddressStatus(in.Status.ExternalAddress),
		BuildStatus:  mapBuildStatus(in.Status.CurrentBuild),
	}
	switch {
	case availableCond == nil:
		curr.Available = false
	case availableCond.Status == metav1.ConditionTrue:
		curr.Available = true
		curr.Message = availableCond.Message
		curr.Reason = availableCond.Reason
	default:
		curr.Available = false
		curr.Message = availableCond.Message
		curr.Reason = availableCond.Reason
	}
	return curr
}

func mapExternalAddressesToAddressStatus(in []workspacev1alpha1.ExternalAddress) []userworkspace.Address {
	res := []userworkspace.Address{}
	for _, addr := range in {
		res = append(res, userworkspace.Address{
			Port: int(addr.TargetPort),
			Url:  addr.Address,
		})
	}
	return res
}

func mapBuildStatus(in *workspacev1alpha1.BuildStatus) *userworkspace.BuildStatus {
	if in == nil || in.Available == nil {
		return nil
	}
	res := &userworkspace.BuildStatus{}
	res.Completed = *in.Available
	res.BuildName = in.Name
	res.Message = *in.Message
	res.Reason = *in.Reason
	res.SourceHash = in.SourceHash
	return res
}
