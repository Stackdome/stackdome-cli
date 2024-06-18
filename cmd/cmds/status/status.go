package status

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var statusArgs struct {
	voyagerFilePath string
	all             bool
}

func NewStatusCommand() *cobra.Command {
	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Get the status of a resource/all resources",
		Long:  `Get the status of a resource/all resources. Pass --all or -a flag to print the status of all resources.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := status(context.Background(), args); err != nil {
				fmt.Printf("status error: %s \n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.RangeArgs(0, 1),
	}
	statusCmd.Flags().StringVar(&statusArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	statusCmd.Flags().BoolVarP(&statusArgs.all, common.AllResourcesFlag, "a", false, fmt.Sprintf("--%s", common.AllResourcesFlag))
	return statusCmd
}

func status(ctx context.Context, args []string) error {
	if !statusArgs.all && len(args) == 0 {
		return fmt.Errorf("no resources specified. Run status <resourceName> to get the status of the resource.")
	}
	userWorkspace, err := common.UserWorkspace(statusArgs.voyagerFilePath)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// Provider initialized.
	currSession, err := session.NewSession(cfg, true)
	if err != nil {
		return err
	}

	if err := userWorkspace.Process(); err != nil {
		return err
	}

	if err := common.ValidateResourceNameRef(args, userWorkspace, statusArgs.all); err != nil {
		return err
	}

	handler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}

	if statusArgs.all {
		args = []string{"all"}
	}

	workspaceStatus, err := handler.Status(ctx, args[0])
	if err != nil {
		return err
	}
	if statusArgs.all {
		printWorkspaceStatus(workspaceStatus)
	} else {
		printResourceStatus(workspaceStatus.ResourceStatuses[0])
	}
	return nil
}

func printWorkspaceStatus(ws *userworkspace.WorkspaceStatus) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"WORKSPACENAME", "AVAILABLE", "REASON", "MESSAGE"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	available := "True"
	if !ws.WorkspaceAvailablityStatus.Available {
		available = "False"
	}
	message := ws.WorkspaceAvailablityStatus.Message
	if message == "" {
		message = "-"
	} else {
		message = strings.ReplaceAll(message, "\n", " ")
	}
	table.Append([]string{ws.WorkspaceName, available, ws.WorkspaceAvailablityStatus.Reason, message})

	table.Render()
	fmt.Println()

	table = tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"RESOURCENAME", "AVAILABLE", "REASON", "MESSAGE", "ADDRESSES", "BUILD STATUS"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	for _, rs := range ws.ResourceStatuses {
		available := "True"
		if !rs.Available {
			available = "False"
		}
		reason := rs.Reason
		if reason == "" {
			reason = "-"
		}
		message := rs.Message
		if message == "" {
			message = "-"
		} else {
			message = strings.ReplaceAll(message, "\n", " ")
		}
		var addresses []string
		for _, addr := range rs.Addresses {
			addresses = append(addresses, fmt.Sprintf("%d<-%s", addr.Port, addr.Url))
		}
		addressStr := "-"
		if len(addresses) > 0 {
			addressStr = strings.Join(addresses, ";")
		}
		buildStatus := "-"
		if rs.BuildStatus != nil {
			buildStatus = fmt.Sprintf("%s (%s)", rs.BuildStatus.BuildName, strings.ReplaceAll(rs.BuildStatus.Message, "\n", " "))
			if !rs.BuildStatus.Completed {
				buildStatus = fmt.Sprintf("%s (Incomplete)", buildStatus)
			}
		}
		table.Append([]string{rs.ResourceName, available, reason, message, addressStr, buildStatus})
	}

	table.Render()
	fmt.Println()

	table = tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"VOLUMENAME", "LOCAL PATH", "LAST SYNCED AT", "AVAILABLE"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	for _, vs := range ws.VolumeStatuses {
		localPath := "-"
		if vs.LocalPath != nil {
			localPath = *vs.LocalPath
		}
		lastSyncedAt := "-"
		if vs.LastSyncedAt != nil {
			lastSyncedAt = *vs.LastSyncedAt
		}
		available := "True"
		if !vs.Available {
			available = "False"
		}
		table.Append([]string{vs.VolumeName, localPath, lastSyncedAt, available})
	}

	table.Render()
}

func printResourceStatus(rs userworkspace.ResourceStatus) {
	fmt.Printf("Resource Status: %s\n\n", rs.ResourceName)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"PROPERTY", "DETAILS"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	table.Append([]string{"Name", rs.ResourceName})
	table.Append([]string{"Availability", formatBool(rs.Available)})
	table.Append([]string{"Reason", formatString(rs.Reason)})
	table.Append([]string{"Message", formatString(rs.Message)})

	var addresses []string
	for _, addr := range rs.Addresses {
		addresses = append(addresses, fmt.Sprintf("%d <- %s", addr.Port, addr.Url))
	}
	table.Append([]string{"Addresses", formatStringSlice(addresses)})

	if rs.BuildStatus != nil {
		table.Append([]string{"Build Information", ""})
		table.Append([]string{"  Name", formatString(rs.BuildStatus.BuildName)})
		table.Append([]string{"  Completed", formatBool(rs.BuildStatus.Completed)})
		table.Append([]string{"  Reason", formatString(rs.BuildStatus.Reason)})
		table.Append([]string{"  Message", formatString(rs.BuildStatus.Message)})
	}

	table.Render()
}

func formatBool(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

func formatString(value string) string {
	if value == "" {
		return "-"
	}
	return strings.ReplaceAll(value, "\n", " ")
}

func formatStringSlice(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

func strPtr(s string) *string {
	return &s
}

func validateResourceNameRef(resourceName string, ws *userworkspace.Workspace) error {
	if resourceName == "all" {
		return nil
	}
	if _, found := ws.Resources[resourceName]; !found {
		return fmt.Errorf("resource '%s' not found in voyagerfile.[Please enter a valid resource defined in the voyagerfile or 'all']", resourceName)
	}
	return nil
}
