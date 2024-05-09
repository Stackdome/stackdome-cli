package deploy

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var deployArgs struct {
	voyagerFilePath string
}

// deployCmd represents the deploy command

func NewDeployCommand() *cobra.Command {
	var deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the resources specified in the voyagerfile",
		Long:  `Deploy the resources specified in the voyagerfile`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := deploy(context.Background()); err != nil {
				fmt.Printf("deploy cmd errored: %s\n", err.Error())
			}
		},
		Args: cobra.NoArgs,
	}
	deployCmd.Flags().StringVar(&deployArgs.voyagerFilePath, "voyagerfile-path", "", "--voyagerfile-path=voyagerfile.yaml")
	return deployCmd
}

func deploy(ctx context.Context) error {
	var voyagerFilePath string
	if len(deployArgs.voyagerFilePath) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		voyagerFilePath, err = common.FindVoyagerFile(cwd)
		if err != nil {
			return err
		}
	} else {
		voyagerFilePath = deployArgs.voyagerFilePath
	}
	if len(voyagerFilePath) == 0 {
		return fmt.Errorf("voyager file missing")
	}
	_, err := os.Stat(voyagerFilePath)
	if err != nil {
		return fmt.Errorf("failed to stat voyagerfile at %s: %w", voyagerFilePath, err)
	}

	if err := userworkspace.Validate(voyagerFilePath); err != nil {
		return fmt.Errorf("voyager file not valid: %w", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// Provider initialized.
	currSession, err := session.NewSessionWithProvider(cfg)
	if err != nil {
		return err
	}

	userWorkspace, err := userworkspace.Unmarshal(voyagerFilePath)
	if err != nil {
		return err
	}
	if err := userWorkspace.Process(); err != nil {
		return err
	}
	userWorkspace.SetAsReady()
	userWorkspace.SetDirHashForAllResources()
	PrintHashAndSyncStatus(*userWorkspace)
	handler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}

	return handler.Deploy(ctx)
}

func PrintHashAndSyncStatus(in userworkspace.Workspace) {
	for key, value := range in.Resources {
		if value.Build != nil {
			fmt.Printf("workspace resource:%s, Sourcehash: %s, NeedsSync: %v\n", key, value.Build.DirHash, value.NeedsSync)
		} else {
			fmt.Printf("workspace resource:%s, NeedsSync: %v\n", key, value.NeedsSync)
		}
	}
}
