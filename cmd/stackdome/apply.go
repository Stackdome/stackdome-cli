package main

import (
	"fmt"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/stackfile"
	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
)

type applyOptions struct {
	File string
	Name string
}

func newApplyCmd() *cobra.Command {
	var opts applyOptions

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Save a stack definition without releasing it",
		Long: `Create or update the saved stack definition from a Stackfile or stack JSON.

Apply does not create a release or change the running workload. Use
` + "`stackdome create release`" + ` to release the saved definition, or use
` + "`stackdome deploy`" + ` to apply and release in one command.`,
		Example: "  stackdome apply -f stackfile.yaml\n  stackdome apply -f stack.json --name demo -o json",
		Args:    cobra.NoArgs,
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, _ []string) error {
			result, err := applyStackDefinition(ctx, cmd, opts)
			if err != nil {
				return err
			}
			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(result)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Stack %q saved. No release was created.\n", result.Name)
			fmt.Fprintf(cmd.ErrOrStderr(), "Release it with: stackdome create release --stack %s\n", result.Name)
			return nil
		})),
	}

	cmd.Flags().StringVarP(&opts.File, "file", "f", "stackfile.yaml", "Path to stackfile or stack JSON")
	cmd.Flags().StringVar(&opts.Name, "name", "", "Override stack name")
	return cmd
}

func applyStackDefinition(ctx *cmdutil.CommandContext, cmd *cobra.Command, opts applyOptions) (*openapi.Stack, error) {
	stack, err := loadStack(opts.File, opts.Name)
	if err != nil {
		return nil, err
	}
	if err := stackfile.ResolveStack(cmd.Context(), stack, &apiResolver{c: ctx.Client}); err != nil {
		return nil, err
	}
	result, err := ctx.Client.ApplyStack(cmd.Context(), *stack)
	if err != nil {
		return nil, err
	}
	if err := ctx.Config.SetCurrentStack(result.GetId()); err != nil {
		return nil, err
	}
	return result, nil
}
