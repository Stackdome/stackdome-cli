package main

import (
	"fmt"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newSignupCmd() *cobra.Command {
	var (
		flagURL      string
		flagInsecure bool
	)

	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Show web signup and CLI setup instructions",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdutil.GetContext(cmd)
			setup, _, err := resolveAuthSetup(flagURL, flagInsecure, ctx.Config)
			if err != nil {
				return err
			}
			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(setup)
			}
			fmt.Fprintln(cmd.ErrOrStderr(), setup.signupGuidance())
			return nil
		},
	}

	cmd.Flags().StringVar(&flagURL, "url", "", "Stackdome server URL (defaults to the selected instance)")
	cmd.Flags().BoolVar(&flagInsecure, "insecure", false, "Allow HTTP or skip HTTPS certificate verification")

	return cmd
}
