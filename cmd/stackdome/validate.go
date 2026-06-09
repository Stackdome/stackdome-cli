package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/stackfile"
)

func newValidateCmd() *cobra.Command {
	var flagFile string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a stackfile",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := stackfile.Load(flagFile)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Stackfile %q is valid.\n", flagFile)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flagFile, "file", "f", "stackfile.yaml", "Path to stackfile")

	return cmd
}
