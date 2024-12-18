package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/mapper"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/tools"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
)

type WorkspaceService interface {
	CreateWorkspace(ctx context.Context, stack *v1alpha1.UserStack) (*v1alpha1.Workspace, *ServiceError)
	GetCurrentWorkspaces(ctx context.Context) ([]*v1alpha1.Workspace, *ServiceError)
	GetWorkspace(ctx context.Context, id string) (*v1alpha1.Workspace, *ServiceError)
	GetWorkspaceByName(ctx context.Context, name string) (*v1alpha1.Workspace, *ServiceError)
	UpdateWorkspace(ctx context.Context, ID string, workspace *v1alpha1.UserStack) (*v1alpha1.Workspace, *ServiceError)
	DeleteWorkspace(ctx context.Context, ID string) *ServiceError
	ListWorkspaceResources(ctx context.Context, workspaceID string) ([]v1alpha1.WorkspaceResource, *ServiceError)
	TriggerBuildForAllResources(ctx context.Context, workspace *v1alpha1.Workspace) *ServiceError
	TriggerBuildForResource(ctx context.Context, workspace *v1alpha1.Workspace, resourceName string) *ServiceError
	ListWorkspaceBuilds(ctx context.Context, workspace *v1alpha1.Workspace) ([]v1alpha1.ResourceBuild, *ServiceError)
	ListWorkspaceResourceBuilds(ctx context.Context, workspace *v1alpha1.Workspace, resource *v1alpha1.WorkspaceResource) ([]v1alpha1.ResourceBuild, *ServiceError)
	RestartAllResources(ctx context.Context, workspace *v1alpha1.Workspace) *ServiceError
	RestartResource(ctx context.Context, workspace *v1alpha1.Workspace, resourceName string) *ServiceError
}

type workspaceService struct {
	session session.Session
}

type WorkspaceServiceSpec struct {
	Session session.Session
}

func NewWorkspaceService(spec WorkspaceServiceSpec) WorkspaceService {
	return &workspaceService{
		session: spec.Session,
	}
}

func (w *workspaceService) CreateWorkspace(ctx context.Context, stack *v1alpha1.UserStack) (*v1alpha1.Workspace, *ServiceError) {
	desiredWorkspace := mapper.WorkspaceFromUserStack(stack)
	// TODO: Compute context directory hash using some logic in the filesystem
	// like last modified time of the files in the directory.
	populateBuildContextHash(&desiredWorkspace)

	createdWorkpace, serr := w.session.CreateWorkspace(ctx, &desiredWorkspace)
	if serr != nil {
		return nil, NewServiceError(serr)
	}
	return createdWorkpace, nil
}

func (w *workspaceService) DeleteWorkspace(ctx context.Context, ID string) *ServiceError {
	err := w.session.DeleteWorkspace(ctx, ID)
	if err != nil {
		return NewServiceError(err)
	}
	return nil
}

func (w *workspaceService) GetCurrentWorkspaces(ctx context.Context) ([]*v1alpha1.Workspace, *ServiceError) {
	workspaces, serr := w.session.GetCurrentWorkspaces(ctx)
	if serr != nil {
		return nil, NewServiceError(serr)
	}
	return workspaces, nil
}

func (w *workspaceService) ListWorkspaceResources(ctx context.Context, workspaceID string) ([]v1alpha1.WorkspaceResource, *ServiceError) {
	resources, serr := w.session.GetWorkspaceResources(ctx, workspaceID)
	if serr != nil {
		return nil, NewServiceError(serr)
	}
	return resources, nil
}

func (w *workspaceService) ListWorkspaceBuilds(ctx context.Context, workspace *v1alpha1.Workspace) ([]v1alpha1.ResourceBuild, *ServiceError) {
	builds, serr := w.session.GetWorkspaceBuilds(ctx, workspace)
	if serr != nil {
		return nil, NewServiceError(serr)
	}
	for i := range builds {
		builds[i].WorkspaceName = workspace.Name
		resource := workspace.GetResourceByName(builds[i].WorkspaceResourceName)
		if resource != nil && resource.BuildConfig != nil && resource.BuildConfig.ContextDirHash == builds[i].SourceHash {
			builds[i].Current = true
		}
	}
	return builds, nil
}

func (w *workspaceService) ListWorkspaceResourceBuilds(ctx context.Context, workspace *v1alpha1.Workspace, resource *v1alpha1.WorkspaceResource) ([]v1alpha1.ResourceBuild, *ServiceError) {
	builds, serr := w.session.GetWorkspaceResourceBuilds(ctx, workspace, resource.Name)
	if serr != nil {
		return nil, NewServiceError(serr)
	}
	for i := range builds {
		builds[i].WorkspaceName = workspace.Name
		if resource.BuildConfig != nil && resource.BuildConfig.ContextDirHash == builds[i].SourceHash {
			builds[i].Current = true
		}
	}
	return builds, nil
}

func (w *workspaceService) GetWorkspace(ctx context.Context, id string) (*v1alpha1.Workspace, *ServiceError) {
	workspace, serr := w.session.GetWorkspace(ctx, id)
	if serr != nil {
		return nil, NewServiceErrorWithCode(serr, serr.HttpCode)
	}
	resources, rErr := w.session.GetWorkspaceResources(ctx, workspace.ID)
	if rErr != nil {
		return nil, NewServiceError(rErr)
	}
	workspace.Resources = resources
	return workspace, nil
}

func (w *workspaceService) GetWorkspaceByName(ctx context.Context, name string) (*v1alpha1.Workspace, *ServiceError) {
	workspaces, serr := w.session.GetCurrentWorkspaces(ctx)
	if serr != nil {
		return nil, NewServiceError(serr)
	}
	for _, ws := range workspaces {
		if ws.Name == name {
			resources, rErr := w.session.GetWorkspaceResources(ctx, ws.ID)
			if rErr != nil {
				return nil, NewServiceError(rErr)
			}
			ws.Resources = resources
			return ws, nil
		}
	}
	return nil, NewServiceErrorWithCode(fmt.Errorf("workspace '%s' not found", name), http.StatusNotFound)
}

func (w *workspaceService) UpdateWorkspace(ctx context.Context, ID string, stack *v1alpha1.UserStack) (*v1alpha1.Workspace, *ServiceError) {
	desiredWorkspace := mapper.WorkspaceFromUserStack(stack)
	existingWorkspace, serr := w.GetWorkspace(ctx, ID)
	if serr != nil {
		return nil, NewServiceErrorWithCode(serr, serr.Code)
	}
	if !equality.Semantic.DeepDerivative(desiredWorkspace, existingWorkspace) {
		copyBuildSourceHash(existingWorkspace, &desiredWorkspace)
		updatedWorkspace, updateErr := w.session.UpdateWorkspace(ctx, existingWorkspace.ID, &desiredWorkspace)
		if updateErr != nil {
			return nil, NewServiceError(updateErr)
		}
		return updatedWorkspace, nil
	}
	return existingWorkspace, nil
}

func (w *workspaceService) TriggerBuildForAllResources(ctx context.Context, existingWorkspace *v1alpha1.Workspace) *ServiceError {
	for _, resource := range existingWorkspace.Resources {
		if resource.BuildConfig != nil {
			resource.BuildConfig.ContextDirHash = tools.GenRandomHash()
		}
	}
	_, err := w.session.UpdateWorkspace(ctx, existingWorkspace.ID, existingWorkspace)
	if err != nil {
		return NewServiceError(err)
	}
	return nil
}

func (w *workspaceService) TriggerBuildForResource(ctx context.Context, existingWorkspace *v1alpha1.Workspace, resourceName string) *ServiceError {
	for i := range existingWorkspace.Resources {
		currResource := &existingWorkspace.Resources[i]
		if currResource.Name == resourceName && currResource.BuildConfig == nil {
			return NewServiceError(fmt.Errorf("resource '%s' does not have a build config", resourceName))
		}
		if currResource.Name == resourceName && currResource.BuildConfig != nil {
			currResource.BuildConfig.ContextDirHash = tools.GenRandomHash()
			_, err := w.session.UpdateWorkspace(ctx, existingWorkspace.ID, existingWorkspace)
			if err != nil {
				return NewServiceError(err)
			}
			return nil
		}
	}
	return NewServiceError(fmt.Errorf("resource '%s' not found in workspace", resourceName))
}

func (w *workspaceService) RestartAllResources(ctx context.Context, existingWorkspace *v1alpha1.Workspace) *ServiceError {
	for i := range existingWorkspace.Resources {
		currResource := &existingWorkspace.Resources[i]
		currResource.LifecycleConfig = &v1alpha1.LifecycleConfig{
			RestartRequestTime: ptr.To(time.Now().UTC().Round(time.Second)),
		}
	}
	_, err := w.session.UpdateWorkspace(ctx, existingWorkspace.ID, existingWorkspace)
	if err != nil {
		return NewServiceError(err)
	}
	return nil
}

func (w *workspaceService) RestartResource(ctx context.Context, existingWorkspace *v1alpha1.Workspace, resourceName string) *ServiceError {
	for i := range existingWorkspace.Resources {
		currResource := &existingWorkspace.Resources[i]
		if currResource.Name == resourceName {
			currResource.LifecycleConfig = &v1alpha1.LifecycleConfig{
				RestartRequestTime: ptr.To(time.Now().UTC().Round(time.Second)),
			}
			_, err := w.session.UpdateWorkspace(ctx, existingWorkspace.ID, existingWorkspace)
			if err != nil {
				return NewServiceError(err)
			}
			return nil
		}
	}
	return NewServiceError(fmt.Errorf("resource '%s' not found in workspace", resourceName))
}

func copyBuildSourceHash(existingWS, desiredWS *v1alpha1.Workspace) {
	currentBuildHashMap := make(map[string]string)

	for i := range existingWS.Resources {
		currResource := &existingWS.Resources[i]
		if currResource.BuildConfig != nil {
			currentBuildHashMap[currResource.Name] = currResource.BuildConfig.ContextDirHash
		}
	}
	for i := range desiredWS.Resources {
		currResource := &desiredWS.Resources[i]
		if currResource.BuildConfig != nil {
			if _, found := currentBuildHashMap[currResource.Name]; found {
				currResource.BuildConfig.ContextDirHash = currentBuildHashMap[currResource.Name]
			} else {
				// This is a new resource, so we need to set a new build hash.
				currResource.BuildConfig.ContextDirHash = tools.GenRandomHash()
			}
		}
	}
}

func populateBuildContextHash(workspace *v1alpha1.Workspace) {
	for i := range workspace.Resources {
		currResource := &workspace.Resources[i]
		if currResource.BuildConfig != nil {
			currResource.BuildConfig.ContextDirHash = tools.GenRandomHash()
		}
	}
}
