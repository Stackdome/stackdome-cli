package main

import (
	"os"

	"github.com/ashishmax31/voyager-cli/cmd/cmds/build"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/delete"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/deploy"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/exec"
	initcmd "github.com/ashishmax31/voyager-cli/cmd/cmds/init"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/list"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/login"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/logs"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/restart"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/status"
	synccmd "github.com/ashishmax31/voyager-cli/cmd/cmds/sync"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/syncsession"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/validate"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var logLevel string

func main() {
	rootCmd := &cobra.Command{
		Use:   "voyager-cli [--log-level=debug|info|warn|error|fatal|panic]",
		Short: "CLI to manage, lifecycle and interact with your applications deployed on voyager stack",
		Long:  `CLI to manage, lifecycle and interact with your applications deployed on voyager stack`,
	}
	buildCmd := build.NewBuildCommand()
	deployCmd := deploy.NewDeployCommand()
	initCmd := initcmd.NewInitCommand()
	loginCmd := login.NewLoginCommand()
	syncCmd := synccmd.NewSyncCommand()
	syncSessionCmd := syncsession.NewSyncSessionCommand()
	validateCmd := validate.NewValidateCommand()
	restartCmd := restart.NewRestartCommand()
	statusCmd := status.NewStatusCommand()
	execCmd := exec.NewExecCommand()
	logsCmd := logs.NewLogsCommand()
	deleteWorkspaceCmd := delete.NewWorkspaceDeleteCommand()
	listCmd := list.NewListCommand()
	rootCmd.AddCommand(
		buildCmd,
		deployCmd,
		initCmd,
		loginCmd,
		syncCmd,
		validateCmd,
		syncSessionCmd,
		restartCmd,
		statusCmd,
		execCmd,
		logsCmd,
		deleteWorkspaceCmd,
		listCmd,
	)
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Set the log level (debug, info, warn)")
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatalf("Invalid log level: %v", err)
	}
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	if err := rootCmd.Execute(); err != nil {
		println(err)
		os.Exit(1)
	}
}
