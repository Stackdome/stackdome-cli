package validate

import (
	"fmt"
	"os"

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

func NewValidateCommand() *cobra.Command {
	var validateCmd = &cobra.Command{
		Use:   "validate",
		Short: "Validate voyagerfile",
		Long:  `Validate voyagerfile.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := validateRun(cmd, args); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
		},
	}
	return validateCmd
}

func validateRun(cmd *cobra.Command, args []string) error {
	voyagerfilePath := args[0]
	err := userworkspace.Validate(voyagerfilePath)
	if err != nil {
		return fmt.Errorf("failed to validate voyagerfile at '%s': %w\n", voyagerfilePath, err)
	}
	return nil
}
