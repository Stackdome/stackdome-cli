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
	"k8s.io/utils/ptr"
)

type listBuildArgs struct {
	resourceName string
}

var listBuildArg listBuildArgs

func newListBuildsCommand() *cobra.Command {
	var listWorkspaceStorageCmd = &cobra.Command{
		Use:   "builds [--resource-name resourceName | -r resourceName]",
		Short: "List created builds in the workspace",
		Long:  `List created builds in the workspace`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  listWorkspaceBuilds,
	}
	listWorkspaceStorageCmd.Flags().StringVarP(&listBuildArg.resourceName, "resource-name", "r", "", "resource name")
	return listWorkspaceStorageCmd
}

func listWorkspaceBuilds(cmd *cobra.Command, args []string) error {
	configArgs := config.Args{}

	if len(listBuildArg.resourceName) == 0 {
		configArgs.AllResources = ptr.To(true)
	} else {
		configArgs.ResourceName = &listBuildArg.resourceName
	}

	runtime, err := config.NewRuntime("list builds", configArgs)
	if err != nil {
		fmt.Printf("list builds error: %s \n", err.Error())
		os.Exit(1)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		fmt.Printf("list builds error: %s \n", err.Error())
		os.Exit(1)
	}

	builds, err := handler.ListWorkspaceBuilds(context.Background(), runtime)
	if err != nil {
		fmt.Printf("list builds error: %s \n", err.Error())
		os.Exit(1)
	}

	printWorkspaceBuilds(builds)
	return nil
}

func printWorkspaceBuilds(builds []v1alpha1.ResourceBuild) {
	// Create a tab writer for formatted output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	defer w.Flush()

	// Print the header
	fmt.Fprintln(w, "ID\tWorkspaceName\tResourceName\tState\tSourceHash")

	// Iterate over the ResourceBuilds and print each
	for _, build := range builds {
		// Extract the state and image URL safely in case Status is nil
		state := "Unknown"
		if build.Status != nil {
			state = build.Status.State
		}

		// Print the fields in a tab-separated format
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			build.ID,
			build.WorkspaceName,
			build.WorkspaceResourceName,
			state,
			renderSourceHash(build),
		)
	}
}

func renderSourceHash(build v1alpha1.ResourceBuild) string {
	if build.Current {
		return fmt.Sprintf("%s (current)", build.SourceHash)
	}
	return build.SourceHash
}
