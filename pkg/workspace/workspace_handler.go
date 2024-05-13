package workspace

import (
	"context"
	"fmt"

	voyagerfile "github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
	"github.com/ashishmax31/voyager-cli/pkg/provider"
	"github.com/ashishmax31/voyager-cli/pkg/provider/k8s"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/sync"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

type WorkspaceHandler struct {
	session              session.Session
	userdefinedWorkspace voyagerfile.Workspace
	syncHandler          sync.Syncer
	provider             provider.Provider
}

func NewWorkspaceStorageHandler(session session.Session, userdefinedWorkspace voyagerfile.Workspace) (*WorkspaceHandler, error) {
	configDir, err := session.Config().ConfigDir()
	if err != nil {
		return nil, err
	}

	provider := k8s.NewK8sProvider(session.Config(), *session.ProviderClient())
	w := &WorkspaceHandler{
		session:              session,
		userdefinedWorkspace: userdefinedWorkspace,
		provider:             provider,
		syncHandler:          sync.NewMutagenSyncer(session.Config(), configDir, "/Users/ashishanand/.voyager/bin", provider),
	}
	return w, nil
}

func (w *WorkspaceHandler) getWorkspace(ctx context.Context, ws *workspacev1alpha1.Workspace) (*workspacev1alpha1.Workspace, bool, error) {
	existing := &workspacev1alpha1.Workspace{}
	if err := w.session.GetResourceFromProvider(ctx,
		types.NamespacedName{Name: ws.Name, Namespace: ws.Namespace},
		existing,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return existing, true, nil
}

func (w *WorkspaceHandler) getWorkspaceStorage(ctx context.Context, desiredWStorage *workspacev1alpha1.WorkspaceStorage) (*workspacev1alpha1.WorkspaceStorage, bool, error) {
	existingWStorage := &workspacev1alpha1.WorkspaceStorage{}
	if err := w.session.GetResourceFromProvider(
		ctx,
		types.NamespacedName{Name: desiredWStorage.Name, Namespace: desiredWStorage.Namespace},
		existingWStorage); err != nil {
		return nil, false, err
	}
	return existingWStorage, true, nil
}

func (w *WorkspaceHandler) getWorkspaceResource(ctx context.Context, resourceName string) (*workspacev1alpha1.WorkspaceResource, error) {
	referencedResource := &workspacev1alpha1.WorkspaceResource{}
	if err := w.session.GetResourceFromProvider(
		ctx,
		types.NamespacedName{Name: resourceName, Namespace: w.session.Config().ProviderConfig.Namespace},
		referencedResource); err != nil {
		return nil, err
	}
	return referencedResource, nil
}

func workspaceHandlerErr(errString string, args ...any) error {
	return fmt.Errorf(errString, args...)
}
