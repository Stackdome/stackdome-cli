package login

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/pkg/client"
	"github.com/ashishmax31/voyager-cli/pkg/config"
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
		Run: func(cmd *cobra.Command, args []string) {
			if err := login(); err != nil {
				fmt.Printf("failed to login: %s \n", err.Error())
				os.Exit(1)
			}
		},
	}
	loginCmd.Flags().StringVar(&args.token, "token", "", "Access token obtained from voyager website")
	loginCmd.Flags().StringVar(&args.voyagerServerUrl, "url", "", "Voyager server url")
	loginCmd.Flags().BoolVar(&args.insecure, "insecure", false, "Voyager server insecure")
	return loginCmd
}

func login() error {
	if args.token == "" {
		return fmt.Errorf("missing token, pass token as --token=<token> flag")
	}
	if args.voyagerServerUrl == "" {
		return fmt.Errorf("missing Voyager server url, pass url as --url=<url> flag")
	}

	cfg := config.New()
	ctx := context.Background()
	cfg.AccessToken = args.token
	cfg.VoyagerServerUrl = args.voyagerServerUrl
	cfg.Insecure = args.insecure

	voyagerClient := client.NewVoyagerServerClient(args.token, args.voyagerServerUrl, args.insecure)
	resp, err := voyagerClient.GetUserInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to authenticate with voyager server: %w", err)
	}
	cfg.Username = resp.Username
	cfg.Organisation = resp.Organisation
	cfg.TokenValidity = resp.TokenValidTill

	providerResp, err := voyagerClient.InitializeProvider(ctx)
	if err != nil {
		return err
	}

	if cfg.ProviderConfig == nil {
		cfg.ProviderConfig = &config.ComputeProviderConfig{}
	}
	cfg.ProviderConfig.CaCert = providerResp.Cacrt
	cfg.ProviderConfig.Namespace = providerResp.Namespace
	cfg.ProviderConfig.Token = providerResp.Token
	cfg.ProviderConfig.ServerUrl = providerResp.ServerUrl
	cfg.UserPrivateKeyPath = "/Users/ashishanand/.voyager/id_rsa"
	cfg.ProviderConfig.SSHUserName = providerResp.SSHUser
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Printf("sucessfully logged in as user: %s \n", cfg.Username)
	return nil
}
