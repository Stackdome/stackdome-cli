package main

import (
	"fmt"
	"os"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/stackfile"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var flagFile string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a Stackfile",
		RunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			sf, err := stackfile.Load(flagFile)
			if err != nil {
				return err
			}
			if _, err := sf.ToStack(); err != nil {
				return clierrors.ValidationError(err.Error())
			}
			result := struct {
				Valid bool   `json:"valid"`
				File  string `json:"file"`
			}{Valid: true, File: flagFile}
			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(result)
			}
			fmt.Fprintf(os.Stderr, "Stackfile %q is valid.\n", flagFile)
			return nil
		}),
	}

	cmd.Flags().StringVarP(&flagFile, "file", "f", "stackfile.yaml", "Path to Stackfile")

	return cmd
}
