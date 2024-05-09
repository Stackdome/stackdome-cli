package login

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/spf13/cobra"
)

var args struct {
	token            string
	voyagerServerUrl string
	insecure         bool
}

// loginCmd represents the login command

func NewLoginCommand() *cobra.Command {
	var loginCmd = &cobra.Command{
		Use:   "login",
		Short: "Login to a voyager server",
		Long:  `Login to a voyager server, pass the voyager server url and voyager token as args`,
		Args:  cobra.NoArgs,
		RunE:  login,
	}
	loginCmd.Flags().StringVar(&args.token, "token", "", "Access token obtained from voyager website")
	loginCmd.Flags().StringVar(&args.voyagerServerUrl, "url", "", "Voyager server url")
	loginCmd.Flags().BoolVar(&args.insecure, "insecure", false, "Voyager server insecure")
	return loginCmd
}

func login(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to intialize config: %w", err)
	}
	ctx := context.Background()
	cfg.AccessToken = args.token
	cfg.VoyagerServerUrl = args.voyagerServerUrl
	cfg.Insecure = args.insecure
	session := session.NewSession(cfg)
	resp, err := session.Authenticate(ctx)
	if err != nil {
		return fmt.Errorf("failed to authenticate with voyager server: %w", err)
	}
	cfg.Username = resp.Username
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}
