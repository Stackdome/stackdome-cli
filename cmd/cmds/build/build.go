package build

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

var buildArgs struct {
	voyagerFilePath string
}

func NewBuildCommand() *cobra.Command {
	var buildCmd = &cobra.Command{
		Use:   "build",
		Short: "Trigger a new build for a resource/all resources.",
		Long:  `Trigger a new build for a resource/all resources.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := build(context.Background(), args); err != nil {
				fmt.Printf("build command errored: %s \n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.ExactArgs(1),
	}
	buildCmd.Flags().StringVar(&buildArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return buildCmd
}

func build(ctx context.Context, args []string) error {
	userWorkspace, err := common.UserWorkspace(buildArgs.voyagerFilePath)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// Provider initialized.
	currSession, err := session.NewSession(cfg)
	if err != nil {
		return err
	}

	if err := userWorkspace.Process(); err != nil {
		return err
	}

	resourceToBuild := args[0]
	if resourceToBuild == "all" {
		userWorkspace.SetDirHashForAllResources()
	} else {
		concernedResource, ok := userWorkspace.Resources[resourceToBuild]
		if !ok {
			return fmt.Errorf("build error: specified resource not found in the voyager file")
		}
		concernedResource.SetDirHash()
	}

	handler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}
	return handler.Deploy(ctx)
}
