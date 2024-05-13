package logs

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var logsArgs struct {
	voyagerFilePath string
	all             bool
	tailLines       int64
	follow          bool
}

func NewLogsCommand() *cobra.Command {
	var logsCmd = &cobra.Command{
		Use:   "logs",
		Short: "Get the logs of a resource",
		Long:  `Get the logs of a resource. Pass --all or -a flag to print the logs of all resources.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := logs(context.Background(), args); err != nil {
				fmt.Printf("exec error: %s \n", err.Error())
				os.Exit(1)
			}
		},
	}
	logsCmd.Flags().BoolVarP(&logsArgs.all, "all", "a", false, "-a or --all")
	logsCmd.Flags().BoolVarP(&logsArgs.follow, "follow", "f", false, "-f or --follow")
	logsCmd.Flags().Int64VarP(&logsArgs.tailLines, "tail", "t", 100, "-t=10 or --tail=10")
	logsCmd.Flags().StringVar(&logsArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return logsCmd
}

func logs(ctx context.Context, args []string) error {
	userWorkspace, err := common.UserWorkspace(logsArgs.voyagerFilePath)
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

	var resourceRef string
	if logsArgs.all {
		resourceRef = "all"
	} else {
		resourceRef = args[0]
	}
	if err := validateResourceNameRef(resourceRef, userWorkspace); err != nil {
		return err
	}
	handler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	signalTermination := make(chan os.Signal, 1)
	signal.Notify(signalTermination, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalTermination
		cancelFn()
	}()
	return ignoreCtxCancelledErr(handler.GetLogs(ctx, resourceRef, logsArgs.follow, logsArgs.tailLines))
}

func ignoreCtxCancelledErr(err error) error {
	if err == context.Canceled {
		return nil
	}
	return err
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
