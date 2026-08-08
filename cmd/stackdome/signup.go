package main

import (
	"github.com/Stackdome/stackdome-cli/internal/client"
	"github.com/Stackdome/stackdome-cli/internal/config"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/spf13/cobra"
)

func newSignupCmd() *cobra.Command {
	var (
		flagURL      string
		flagName     string
		flagEmail    string
		flagPassword string
		flagOrg      string
		flagInsecure bool
	)

	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Create a new Stackdome account",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagURL == "" {
				return clierrors.ValidationError("--url is required")
			}

			if flagName == "" {
				flagName = promptInput("Name: ")
			}
			if flagEmail == "" {
				flagEmail = promptInput("Email: ")
			}
			if flagPassword == "" {
				flagPassword = promptPassword("Password: ")
			}
			if flagOrg == "" {
				flagOrg = promptInput("Organisation name: ")
			}

			c := client.New(flagURL, client.WithInsecure(flagInsecure))

			result, err := c.Signup(cmd.Context(), flagName, flagEmail, flagPassword, flagOrg)
			if err != nil {
				return err
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			cfg.ServerURL = flagURL
			cfg.AccessToken = result.AccessToken
			cfg.RefreshToken = result.RefreshToken
			cfg.OrganizationID = result.User.GetOrganisationId()
			cfg.Username = userDisplayName(result.User)
			cfg.Insecure = flagInsecure

			if err := persistLogin(cmd, c, cfg); err != nil {
				return err
			}

			return printAuthenticationResult(cmd, cfg, true, "session")
		},
	}

	cmd.Flags().StringVar(&flagURL, "url", "", "Stackdome server URL (required)")
	cmd.Flags().StringVar(&flagName, "name", "", "Your name")
	cmd.Flags().StringVar(&flagEmail, "email", "", "Email address")
	cmd.Flags().StringVar(&flagPassword, "password", "", "Password")
	cmd.Flags().StringVar(&flagOrg, "org", "", "Organisation name")
	cmd.Flags().BoolVar(&flagInsecure, "insecure", false, "Allow insecure HTTPS")

	return cmd
}
