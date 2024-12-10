package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/client"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/provider"
	"github.com/ashishmax31/voyager-cli/pkg/provider/k8s"
	"github.com/ashishmax31/voyager-cli/pkg/services"
	"github.com/ashishmax31/voyager-cli/pkg/sync"
)

type workspaceHandler struct {
	runtime                        *config.Runtime
	syncHandler                    sync.Syncer
	provider                       provider.Provider
	workspaceStorageService        services.WorkspaceStorageService
	workspaceService               services.WorkspaceService
	workspaceInitializationService services.WorkspaceInitializationService
}

func NewWorkspaceHandler(runtime *config.Runtime) (*workspaceHandler, error) {
	w := &workspaceHandler{
		runtime: runtime,
		workspaceStorageService: services.NewWorkspaceStorageService(services.WorkspaceStorageServiceSpec{
			Session: runtime.Session,
		}),
		workspaceService: services.NewWorkspaceService(services.WorkspaceServiceSpec{
			Session: runtime.Session,
		}),
		workspaceInitializationService: services.NewWorkspaceInitializationService(runtime),
	}

	if runtime.Config().ProviderConfigPresent() {
		providerClient, err := client.NewProviderClient(runtime.Config())
		if err != nil {
			return nil, workspaceHandlerErr("failed to create provider client: %w", err)
		}
		provider := k8s.NewK8sProvider(runtime.Config(), providerClient)
		w.provider = provider
		w.syncHandler = sync.NewMutagenSyncer(runtime.Config(), runtime.ConfigDir, runtime.DepsDir, provider)
	}
	return w, nil
}

func (w *workspaceHandler) Initialize(ctx context.Context, workspaceName string) error {
	if err := w.workspaceInitializationService.InitializeWorkspace(ctx, workspaceName); err != nil {
		return workspaceHandlerErr("failed to initialize workspace: %w", err)
	}
	return nil
}

func workspaceHandlerErr(errString string, args ...any) error {
	return fmt.Errorf(errString, args...)
}
