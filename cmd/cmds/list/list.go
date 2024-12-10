package list

import "github.com/spf13/cobra"

func NewListCommand() *cobra.Command {
	var listCmd = &cobra.Command{
		Use:   "list workspaces | workspace-storages",
		Short: "List various resources owned by the user",
		Long:  `List various resources owned by the user`,
		Args:  cobra.NoArgs,
	}

	listCmd.AddCommand(newListWorkspacesCommand(), newListWorkspaceStorageCommand())
	return listCmd
}
