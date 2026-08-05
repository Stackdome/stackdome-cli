package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
	"github.com/stackdome/cli/internal/config"
	clierrors "github.com/stackdome/cli/internal/errors"
	"github.com/stackdome/cli/internal/output"
)

var (
	flagLogLevel string
	flagNoColor  bool
	flagOutput   string
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "stackdome",
		Short:         "CLI for the Stackdome platform",
		Long: `Deploy, manage, and monitor your applications on Stackdome.

Every command runs non-interactively: pass --yes to skip confirmations,
STACKDOME_TOKEN (and STACKDOME_URL) to authenticate without a config file, and
-o json|yaml for machine-readable output. In -o json/yaml mode stdout carries
only the result object; all prose, prompts, and progress go to stderr.

Run ` + "`stackdome whoami`" + ` first to verify credentials.

Exit codes:
  0    success
  1    general error
  2    authentication / authorization failure
  3    not found
  4    invalid input or usage
  5    conflict (already exists, or state does not allow the operation)
  130  canceled (interrupted, or a confirmation was declined)`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			output.SetNoColor(flagNoColor)

			format, err := output.ParseFormat(flagOutput)
			if err != nil {
				return err
			}

			level := parseLogLevel(flagLogLevel)

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			ctx := cmdutil.NewCommandContext(cfg, format, level)
			cmdutil.SetContext(cmd, ctx)
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&flagLogLevel, "log-level", "warn", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "table", "Output format (table, json, yaml)")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newLoginCmd())
	rootCmd.AddCommand(newLogoutCmd())
	rootCmd.AddCommand(newSignupCmd())
	rootCmd.AddCommand(newWhoamiCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newDeployCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newDestroyCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newStackCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newBuildCmd())
	rootCmd.AddCommand(newReleaseCmd())
	rootCmd.AddCommand(newRestartCmd())
	rootCmd.AddCommand(newOpenCmd())
	rootCmd.AddCommand(newSecretCmd())
	rootCmd.AddCommand(newVolumeCmd())
	rootCmd.AddCommand(newAddonCmd())
	rootCmd.AddCommand(newTokenCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newCompletionCmd())

	// Usage errors exit 4, as the help text above promises. Cobra reports bad
	// flags and bad arg counts as plain errors, which would otherwise exit 1.
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return clierrors.ValidationError(err.Error())
	})
	wrapArgErrors(rootCmd)

	return rootCmd
}

func wrapArgErrors(cmd *cobra.Command) {
	for _, sub := range cmd.Commands() {
		wrapArgErrors(sub)
	}
	if cmd.Args == nil {
		return
	}
	inner := cmd.Args
	cmd.Args = func(c *cobra.Command, args []string) error {
		if err := inner(c, args); err != nil {
			return clierrors.ValidationError(err.Error())
		}
		return nil
	}
}

func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	rootCmd := newRootCmd()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		msg := clierrors.UserMessage(err)
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
		return clierrors.ExitCodeFrom(err)
	}
	return 0
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
