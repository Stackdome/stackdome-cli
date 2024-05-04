/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

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
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.NoArgs,
	RunE: login,
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().StringVar(&args.token, "token", "", "Access token obtained from voyager website")
	loginCmd.Flags().StringVar(&args.voyagerServerUrl, "url", "", "Voyager server url")
	loginCmd.Flags().BoolVar(&args.insecure, "insecure", false, "Voyager server insecure")
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
