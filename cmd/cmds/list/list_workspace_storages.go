package list

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

func newListWorkspaceStorageCommand() *cobra.Command {
	var listWorkspaceStorageCmd = &cobra.Command{
		Use:   "workspace-storages",
		Short: "List workspace storages",
		Long:  `List workspace storages`,
		Args:  cobra.NoArgs,
		RunE:  listWorkspaceStorages,
	}
	return listWorkspaceStorageCmd
}

func listWorkspaceStorages(cmd *cobra.Command, args []string) error {
	runtime, err := config.NewRuntime("list workspace storages", config.Args{})
	if err != nil {
		return fmt.Errorf("list workspace storages error: %w", err)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		return fmt.Errorf("list workspace storages error: %w", err)
	}

	storages, err := handler.ListWorkspaceStorages(context.Background(), runtime)
	if err != nil {
		return fmt.Errorf("list workspace storages error: %w", err)
	}

	printWorkspaceStorages(storages)
	return nil
}

func printWorkspaceStorages(storages []*v1alpha1.WorkspaceStorage) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	defer w.Flush()

	fmt.Fprintln(w, "ID\tName\tState\t")
	for _, ws := range storages {
		state := "Unknown"
		if ws.Status != nil {
			state = ws.Status.State
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t\n", ws.ID, ws.Name, state)
	}
}
