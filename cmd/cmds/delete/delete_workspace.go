package delete

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

type deleteFlags struct {
	allWorkspaces          bool
	removeWorkspaceStorage bool
	currentWorkspace       bool
}

var flags deleteFlags

func NewWorkspaceDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "delete-workspace [workspace-name | -a | --all | -c | --current] [--remove-storage] ",
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(cmd, args); err != nil {
				fmt.Printf("delete-workspace error: %s \n", err.Error())
			}
		},
		Short: "Delete a workspace or all workspaces",
		Long:  `Delete a workspace or all workspaces.`,
		Args: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			current, _ := cmd.Flags().GetBool("current")

			if len(args) == 0 && !all && !current {
				return fmt.Errorf("you must specify a workspace name, use --all/-a, or use --current/-c")
			}

			if (all && current) || (len(args) > 0 && (all || current)) {
				return fmt.Errorf("you cannot specify a workspace name and use --all/-a or --current/-c together")
			}

			return nil
		},
	}
	cmd.Flags().BoolVarP(&flags.allWorkspaces, "all", "a", false, "Delete all workspaces")
	cmd.Flags().BoolVar(&flags.removeWorkspaceStorage, "remove-storage", false, "Remove workspace storage")
	cmd.Flags().BoolVarP(&flags.currentWorkspace, "current", "c", false, "Delete the current workspace")
	return cmd
}

func run(_ *cobra.Command, args []string) error {
	// Append a default value to args if none is provided. This wont be used in the delete-workspace command.
	if len(args) == 0 {
		args = append(args, "")
	}

	runtime, err := config.NewRuntime("delete-workspace", config.Args{
		WorkspaceName:    &args[0],
		AllWorkspaces:    &flags.allWorkspaces,
		RemoveStorage:    &flags.removeWorkspaceStorage,
		CurrentWorkspace: &flags.currentWorkspace,
	})

	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		return fmt.Errorf("failed to create workspace handler: %w", err)
	}

	return handler.Delete(context.Background(), runtime)
}
