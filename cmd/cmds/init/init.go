package init

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"

	"github.com/spf13/cobra"
)

var initArgs struct {
	workspaceName string
}

// initCmd represents the init command
func NewInitCommand() *cobra.Command {
	var initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize workspace environment",
		Long:  `Initialize workspace environment`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(); err != nil {
				fmt.Printf("init error: %s \n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.NoArgs,
	}
	// Required flag workspace-name
	initCmd.Flags().StringVar(&initArgs.workspaceName, "workspace-name", "", "workspace name")
	return initCmd
}

func run() error {
	if initArgs.workspaceName == "" {
		return fmt.Errorf("workspace name is required")
	}

	runtime, err := config.NewRuntime("init", config.Args{
		WorkspaceName: &initArgs.workspaceName,
	})

	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		return fmt.Errorf("failed to create workspace handler: %w", err)
	}

	if err := handler.Initialize(context.Background(), initArgs.workspaceName); err != nil {
		return fmt.Errorf("failed to initialize workspace: %w", err)
	}

	return nil
}
