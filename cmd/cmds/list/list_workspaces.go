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

func newListWorkspacesCommand() *cobra.Command {
	var listWorkspacesCmd = &cobra.Command{
		Use:   "workspaces",
		Short: "List all workspaces",
		Long:  `List all workspaces`,
		Args:  cobra.NoArgs,
		RunE:  listWorkspaces,
	}

	return listWorkspacesCmd
}

func listWorkspaces(cmd *cobra.Command, args []string) error {
	runtime, err := config.NewRuntime("list workspaces", config.Args{})
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		return fmt.Errorf("failed to create workspace handler: %w", err)
	}

	workspaces, err := handler.ListWorkspaces(context.Background(), runtime)
	if err != nil {
		return fmt.Errorf("list workspaces error: %w", err)
	}

	printWorkspaces(workspaces)
	return nil
}

func printWorkspaces(workspaces []*v1alpha1.Workspace) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	defer w.Flush()

	fmt.Fprintln(w, "ID\tName\tState\t")

	for _, ws := range workspaces {
		state := "Unknown"
		if ws.Status != nil {
			state = ws.Status.State
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t\n", ws.ID, ws.Name, state)
	}
}
