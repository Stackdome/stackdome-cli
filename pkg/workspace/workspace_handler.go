package workspace

import (
	"fmt"

	voyagerfile "github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
	"github.com/ashishmax31/voyager-cli/pkg/provider"
	"github.com/ashishmax31/voyager-cli/pkg/provider/k8s"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/sync"
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
	w := &WorkspaceHandler{
		session:              session,
		userdefinedWorkspace: userdefinedWorkspace,
		provider:             k8s.NewK8sProvider(session.Config(), *session.ProviderClient()),
		syncHandler:          sync.NewMutagenSyncer(session.Config(), configDir, "/Users/ashishanand/.voyager/bin"),
	}
	return w, nil
}

func workspaceHandlerErr(errString string, args ...any) error {
	return fmt.Errorf(errString, args...)
}
