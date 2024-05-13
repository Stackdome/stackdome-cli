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
}

func NewStatusCommand() *cobra.Command {
	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Get the status of a resource/all resources.",
		Long:  `Get the status of a resource/all resources.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := status(context.Background(), args); err != nil {
				fmt.Printf("build command errored: %s \n", err.Error())
				os.Exit(1)
			}
		},
	}
	statusCmd.Flags().StringVar(&statusArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return statusCmd
}

func status(ctx context.Context, args []string) error {
	userWorkspace, err := common.UserWorkspace(statusArgs.voyagerFilePath)
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

	handler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}
	workspaceStatus, err := handler.Status(ctx, "")
	if err != nil {
		return err
	}
	printWorkspaceStatus(workspaceStatus)
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
		if rs.BuildStatus.BuildName != "" {
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
