package init

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

var initArgs struct {
	voyagerFilePath string
}

// initCmd represents the init command
func NewInitCommand() *cobra.Command {
	var initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize your voyager workspace environment",
		Long:  `Initialize your voyager workspace environment`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(); err != nil {
				fmt.Printf("failed to initialize voyager: %s \n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.NoArgs,
	}
	initCmd.Flags().StringVar(&initArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return initCmd
}

func run() error {
	userWorkspace, err := common.UserWorkspace(initArgs.voyagerFilePath)
	if err != nil {
		return err
	}
	if err := userWorkspace.Process(); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	session, err := session.NewSession(cfg)
	if err != nil {
		return err
	}

	handler, err := workspace.NewWorkspaceStorageHandler(session, *userWorkspace)
	if err != nil {
		return err
	}

	return handler.Init(context.Background())
}
