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

var syncArgs struct {
	voyagerFilePath string
}

func NewSyncCommand() *cobra.Command {
	var syncCmd = &cobra.Command{
		Use:   "sync",
		Short: "sync local directories mentioned in the voyagerfile against remote volumes",
		Long:  `sync local directories mentioned in the voyagerfile against remote volumes`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := sync(); err != nil {
				fmt.Printf("failed to sync local directories against remote volumes: %s", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.NoArgs,
	}
	syncCmd.Flags().StringVar(&syncArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return syncCmd
}

func sync() error {
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

	if err := syncHandler.Sync(context.Background()); err != nil {
		return err
	}
	fmt.Printf("Successfully synced volumes..")
	return nil
}
