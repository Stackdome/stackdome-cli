package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
	}

	cmd.AddCommand(newConfigViewCmd())
	cmd.AddCommand(newConfigSetContextCmd())
	cmd.AddCommand(newConfigSetStackCmd())
	return cmd
}

func newConfigViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Show current configuration",
		RunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			if !ctx.Formatter.IsTable() {
				// Redact on a copy: Save() marshals the same struct, so the
				// tokens must stay intact on the real config.
				view := *ctx.Config
				view.AccessToken = redactSecret(view.AccessToken)
				view.RefreshToken = redactSecret(view.RefreshToken)
				return ctx.Formatter.PrintStructured(view)
			}
			fmt.Fprintln(os.Stdout, ctx.Config.Summary())
			return nil
		}),
	}
}

// redactSecret keeps enough of a credential to recognise which one is in play
// without printing anything usable.
func redactSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) > 8 {
		return s[:8] + "..."
	}
	return "..."
}

func newConfigSetStackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-stack <stack-id>",
		Short: "Set the current stack context",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			if err := ctx.Config.SetCurrentStack(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Current stack set to %s\n", args[0])
			return nil
		}),
	}
}

func newConfigSetContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-context <url>",
		Short: "Switch to a different Stackdome server",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			newURL := args[0]
			if newURL == "" {
				return clierrors.ValidationError("URL cannot be empty")
			}

			ctx.Config.ServerURL = newURL
			ctx.Config.AccessToken = ""
			ctx.Config.RefreshToken = ""
			ctx.Config.OrganizationID = ""
			ctx.Config.ProjectName = ""
			ctx.Config.Username = ""
			ctx.Config.CurrentStack = ""

			if err := ctx.Config.Save(); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Context switched to %s. Run `stackdome login` to authenticate.\n", newURL)
			return nil
		}),
	}
}
