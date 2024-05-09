package main

import (
	"os"

	"github.com/ashishmax31/voyager-cli/cmd/cmds/build"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/deploy"
	initcmd "github.com/ashishmax31/voyager-cli/cmd/cmds/init"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/login"
	synccmd "github.com/ashishmax31/voyager-cli/cmd/cmds/sync"
	"github.com/ashishmax31/voyager-cli/cmd/cmds/validate"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "voyager-cli",
		Short: "CLI to manage, lifecycle and interact with your applications deployed on voyager stack",
		Long:  `CLI to manage, lifecycle and interact with your applications deployed on voyager stack`,
	}

	buildCmd := build.NewBuildCommand()
	deployCmd := deploy.NewDeployCommand()
	initCmd := initcmd.NewInitCommand()
	loginCmd := login.NewLoginCommand()
	syncCmd := synccmd.NewSyncCommand()
	validateCmd := validate.NewValidateCommand()

	rootCmd.AddCommand(buildCmd, deployCmd, initCmd, loginCmd, syncCmd, validateCmd)

	if err := rootCmd.Execute(); err != nil {
		println(err)
		os.Exit(1)
	}
}
