package main

import (
	"strings"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	"github.com/spf13/cobra"
)

type whoamiInfo struct {
	User        string `json:"user"`
	Email       string `json:"email,omitempty"`
	Org         string `json:"organization_id"`
	Project     string `json:"project,omitempty"`
	AuthMethod  string `json:"auth_method"`
	TokenSource string `json:"token_source"`
	ServerURL   string `json:"server_url"`
	Stack       string `json:"current_stack,omitempty"`
}

func authDetails(cfg *config.Config) (method, source string) {
	method = "session (jwt)"
	if strings.HasPrefix(cfg.AccessToken, "sdm_") {
		method = "api token"
	}
	source = "config file"
	if cfg.TokenFromEnv() {
		source = "STACKDOME_TOKEN"
	}
	return method, source
}

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the authenticated user, org, project, and auth method",
		Long:  "Show which credentials are in play. Run this first to verify a token works.",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			user, err := ctx.Client.GetCurrentUser(cmd.Context())
			if err != nil {
				return err
			}

			authMethod, tokenSource := authDetails(ctx.Config)

			info := whoamiInfo{
				User:        userDisplayName(user),
				Email:       user.GetEmail(),
				Org:         ctx.Config.OrganizationID,
				Project:     ctx.Config.ProjectName,
				AuthMethod:  authMethod,
				TokenSource: tokenSource,
				ServerURL:   ctx.Config.ServerURL,
				Stack:       ctx.Config.CurrentStack,
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(info)
			}

			f := ctx.Formatter
			f.Printf("User:     %s\n", info.User)
			if info.Email != "" && info.Email != info.User {
				f.Printf("Email:    %s\n", info.Email)
			}
			f.Printf("Org:      %s\n", info.Org)
			if info.Project != "" {
				f.Printf("Project:  %s\n", info.Project)
			}
			f.Printf("Auth:     %s (from %s)\n", info.AuthMethod, info.TokenSource)
			f.Printf("Server:   %s\n", info.ServerURL)
			if info.Stack != "" {
				f.Printf("Stack:    %s\n", info.Stack)
			}
			return nil
		})),
	}
}
