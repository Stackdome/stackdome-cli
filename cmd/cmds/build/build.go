package build

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
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
				fmt.Printf("build cmd errored: %s \n", err.Error())
			}
		},
		Args: cobra.ExactArgs(1),
	}
	buildCmd.Flags().StringVar(&buildArgs.voyagerFilePath, "voyagerfile-path", "", "--voyagerfile-path=voyagerfile.yaml")
	return buildCmd
}

func build(ctx context.Context, args []string) error {
	var voyagerFilePath string
	if len(buildArgs.voyagerFilePath) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		voyagerFilePath, err = common.FindVoyagerFile(cwd)
		if err != nil {
			return err
		}
	} else {
		voyagerFilePath = buildArgs.voyagerFilePath
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
