package sync

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

func NewSyncStatusCommand() *cobra.Command {
	var syncStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Current sync status",
		Long:  `Current sync status`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := syncStatus(); err != nil {
				fmt.Printf("failed to check sync status: %s\n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.NoArgs,
	}
	syncStatusCmd.Flags().StringVar(&syncArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return syncStatusCmd
}

func syncStatus() error {
	userWorkspace, err := common.UserWorkspace(syncArgs.voyagerFilePath)
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

	return syncHandler.SyncStatus(context.Background())
}
