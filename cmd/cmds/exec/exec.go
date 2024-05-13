package exec

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

var execArgs struct {
	interactive     bool
	voyagerFilePath string
}

func NewExecCommand() *cobra.Command {
	var execCmd = &cobra.Command{
		Use:   "exec",
		Short: "Execute a command inside a workspace resource.",
		Long:  `Execute a command inside a workspace resource. Pass -i or --i flag for starting an interactive session.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := exec(context.Background(), args); err != nil {
				fmt.Printf("exec error: %s \n", err.Error())
				os.Exit(1)
			}
		},
	}
	execCmd.Flags().BoolVarP(&execArgs.interactive, "i", "i", false, "-i or --i")
	execCmd.Flags().StringVar(&execArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return execCmd
}

func exec(ctx context.Context, args []string) error {
	userWorkspace, err := common.UserWorkspace(execArgs.voyagerFilePath)
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

	resourceRef := args[0]
	if err := validateResourceNameRef(resourceRef, userWorkspace, false); err != nil {
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
	return ignoreCtxCancelledErr(handler.Execute(ctx, resourceRef, args[1:], execArgs.interactive))
}

func ignoreCtxCancelledErr(err error) error {
	if err == context.Canceled {
		return nil
	}
	return err
}

func validateResourceNameRef(resourceName string, ws *userworkspace.Workspace, allowAll bool) error {
	if resourceName == "all" && allowAll {
		return nil
	}
	if _, found := ws.Resources[resourceName]; !found {
		return fmt.Errorf("resource '%s' not found in voyagerfile.[Please enter a valid resource defined in the voyagerfile or 'all']", resourceName)
	}
	return nil
}
