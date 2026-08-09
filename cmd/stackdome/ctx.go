package main

import (
	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"github.com/spf13/cobra"
)

type ctxStack struct {
	Name string `json:"name" yaml:"name"`
	ID   string `json:"id" yaml:"id"`
}

type ctxInfo struct {
	User           string    `json:"user" yaml:"user"`
	Email          string    `json:"email,omitempty" yaml:"email,omitempty"`
	ServerURL      string    `json:"server_url" yaml:"server_url"`
	OrganizationID string    `json:"organization_id" yaml:"organization_id"`
	Project        string    `json:"project,omitempty" yaml:"project,omitempty"`
	CurrentStack   *ctxStack `json:"current_stack,omitempty" yaml:"current_stack,omitempty"`
	AuthMethod     string    `json:"auth_method" yaml:"auth_method"`
	TokenSource    string    `json:"token_source" yaml:"token_source"`
}

func newCtxCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ctx",
		Short:   "Show the active Stackdome context",
		Long:    "Show the authenticated user, Stackdome server, organization, project, selected stack, and authentication source without revealing credentials.",
		Example: "  stackdome ctx\n  stackdome ctx -o json",
		Args:    cobra.NoArgs,
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, _ []string) error {
			user, err := ctx.Client.GetCurrentUser(cmd.Context())
			if err != nil {
				return err
			}

			info := ctxInfo{
				User:           userDisplayName(user),
				Email:          user.GetEmail(),
				ServerURL:      ctx.Config.ServerURL,
				OrganizationID: ctx.Config.OrganizationID,
				Project:        ctx.Config.ProjectName,
			}
			info.AuthMethod, info.TokenSource = authDetails(ctx.Config)

			if ctx.Config.CurrentStack != "" {
				stacks, err := ctx.Client.ListStacks(cmd.Context())
				if err != nil {
					return err
				}
				for i := range stacks {
					if currentStackMatches(stacks[i], ctx.Config.CurrentStack) {
						info.CurrentStack = &ctxStack{Name: stacks[i].Name, ID: stacks[i].GetId()}
						break
					}
				}
				if info.CurrentStack == nil {
					return clierrors.NotFoundError("Stack", ctx.Config.CurrentStack)
				}
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(info)
			}
			renderCtx(ctx.Formatter, info)
			return nil
		})),
	}
}

func renderCtx(formatter *output.Formatter, info ctxInfo) {
	user := info.User
	if info.Email != "" && info.Email != info.User {
		user += " <" + info.Email + ">"
	}
	formatter.Printf("User:     %s\n", user)
	formatter.Printf("Server:   %s\n", info.ServerURL)
	formatter.Printf("Org:      %s\n", info.OrganizationID)
	if info.Project != "" {
		formatter.Printf("Project:  %s\n", info.Project)
	}
	if info.CurrentStack == nil {
		formatter.Printf("Stack:    none\n")
		formatter.Printf("Select one with: stackdome use stack <stack>\n")
	} else {
		formatter.Printf("Stack:    %s (%s)\n", info.CurrentStack.Name, info.CurrentStack.ID)
	}
	formatter.Printf("Auth:     %s (from %s)\n", info.AuthMethod, info.TokenSource)
}
