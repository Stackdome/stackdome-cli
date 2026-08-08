package main

import (
	"fmt"
	"os"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear stored credentials",
		RunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			if err := ctx.Config.Clear(); err != nil {
				return err
			}
			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(struct {
					LoggedOut bool `json:"logged_out" yaml:"logged_out"`
				}{LoggedOut: true})
			}
			fmt.Fprintln(os.Stderr, "Logged out.")
			return nil
		}),
	}
}
