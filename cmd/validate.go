/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
	"github.com/spf13/cobra"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate voyager file",
	Long:  `Validate voyager file.`,
	Args:  cobra.ExactArgs(1),
	RunE:  validateRun,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func validateRun(cmd *cobra.Command, args []string) error {
	voyagerfilePath := args[0]
	err := userworkspace.Validate(voyagerfilePath)
	if err != nil {
		return fmt.Errorf("failed to validate voyagerfile at '%s': %w\n", voyagerfilePath, err)
	}
	return nil
}
