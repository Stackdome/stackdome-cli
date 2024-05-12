package syncsession

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

func newSyncSessionStopCommand() *cobra.Command {
	var syncCmd = &cobra.Command{
		Use:   "stop",
		Short: "stop a sync sync session",
		Long:  `Stop a sync sync session`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := stopSyncSession(); err != nil {
				fmt.Printf("failed to stop sync session: %s\n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.NoArgs,
	}
	return syncCmd
}

func stopSyncSession() error {
	userWorkspace, err := common.UserWorkspace(syncSessionArgs.voyagerFilePath)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	currSession, err := session.NewSession(cfg)
	if err != nil {
		return err
	}

	if err := userWorkspace.Process(); err != nil {
		return err
	}

	syncHandler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}

	return syncHandler.StopSyncSession(context.Background())
}
