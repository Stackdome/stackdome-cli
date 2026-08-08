package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"github.com/spf13/cobra"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage API tokens",
		Long:  "Manage personal access tokens for headless and CI access.\n\nUse `stackdome token scopes` to discover valid scopes.",
	}

	cmd.AddCommand(newTokenCreateCmd())
	cmd.AddCommand(newTokenListCmd())
	cmd.AddCommand(newTokenDeleteCmd())
	cmd.AddCommand(newTokenScopesCmd())
	return cmd
}

func newTokenCreateCmd() *cobra.Command {
	var (
		flagScopes    []string
		flagResources []string
		flagExpires   string
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an API token",
		Long:  "Create an API token. The token value is shown once and cannot be retrieved again.",
		Example: "  stackdome token create ci --scope 'stacks:*' --scope secrets:read --expires 720h\n" +
			"  stackdome token create agent --scope '*:*' -o json",
		Args: cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			if len(flagScopes) == 0 {
				return clierrors.ValidationError("At least one --scope is required (see `stackdome token scopes`)")
			}
			expiresAt, err := parseExpiry(flagExpires, time.Now())
			if err != nil {
				return err
			}

			token, err := ctx.Client.CreateAPIToken(cmd.Context(), args[0], flagScopes, flagResources, expiresAt)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(token)
			}

			fmt.Fprintf(os.Stderr, "Token %q created. Save it now — it will not be shown again.\n\n", token.GetName())
			fmt.Println(token.GetToken())
			if t := token.GetExpiresAt(); !t.IsZero() {
				fmt.Fprintf(os.Stderr, "\nExpires: %s\n", t.Local().Format(time.RFC3339))
			}
			return nil
		})),
	}

	cmd.Flags().StringArrayVar(&flagScopes, "scope", nil, "Scope as resource:action, e.g. stacks:* (repeatable)")
	cmd.Flags().StringArrayVar(&flagResources, "resource-id", nil, "Restrict the token to specific resource IDs (repeatable)")
	cmd.Flags().StringVar(&flagExpires, "expires", "", "Lifetime as a duration, e.g. 720h (default: never)")

	return cmd
}

func newTokenListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List API tokens",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			tokens, err := ctx.Client.ListAPITokens(cmd.Context())
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(tokens)
			}

			if len(tokens) == 0 {
				fmt.Fprintln(os.Stderr, "No API tokens found.")
				return nil
			}

			tbl := ctx.Formatter.NewTable("ID", "NAME", "PREFIX", "SCOPES", "EXPIRES", "LAST USED")
			for _, t := range tokens {
				tbl.AddRow(
					t.GetId(),
					t.GetName(),
					dashIfEmpty(t.GetTokenPrefix()),
					dashIfEmpty(strings.Join(t.Scopes, ", ")),
					tokenExpiry(t.ExpiresAt),
					tokenLastUsed(t.LastUsedAt),
				)
			}
			tbl.Render()
			return nil
		})),
	}
}

func newTokenDeleteCmd() *cobra.Command {
	var flagYes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an API token",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			if _, err := cmdutil.Confirm(ctx.Formatter, fmt.Sprintf("Delete API token %q?", args[0]), flagYes); err != nil {
				return err
			}

			if err := ctx.Client.DeleteAPIToken(cmd.Context(), args[0]); err != nil {
				return err
			}

			return printMutationResult(ctx, mutationResult{
				Status:   "deleted",
				Resource: "api_token",
				ID:       args[0],
			}, fmt.Sprintf("API token %q deleted.", args[0]))
		})),
	}

	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation")
	return cmd
}

func newTokenScopesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scopes",
		Short: "List valid token scopes",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			scopes, err := ctx.Client.ListTokenScopes(cmd.Context())
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(scopes)
			}

			tbl := ctx.Formatter.NewTable("RESOURCE", "ACTIONS")
			for _, s := range scopes.Items {
				tbl.AddRow(s.GetResource(), strings.Join(s.Actions, ", "))
			}
			tbl.Render()

			if full := scopes.GetFullAccessScope(); full != "" {
				fmt.Fprintf(os.Stderr, "\nFull access: %s\n", full)
			}
			return nil
		})),
	}
}

// parseExpiry turns a --expires duration into an absolute timestamp; empty means
// the token never expires.
func parseExpiry(expires string, now time.Time) (*time.Time, error) {
	if expires == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(expires)
	if err != nil {
		return nil, clierrors.ValidationError(fmt.Sprintf("invalid --expires %q, expected a duration like 720h", expires))
	}
	if d <= 0 {
		return nil, clierrors.ValidationError("--expires must be a positive duration")
	}
	t := now.Add(d)
	return &t, nil
}

// tokenExpiry prints an absolute date — TimeAgo only reads well for the past.
func tokenExpiry(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "never"
	}
	return t.Local().Format(time.RFC3339)
}

func tokenLastUsed(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "never"
	}
	return output.TimeAgo(*t)
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
