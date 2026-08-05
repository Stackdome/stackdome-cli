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
		Long:          "Deploy, manage, and monitor your applications on Stackdome.",
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
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newDeployCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newDestroyCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newStackCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newBuildCmd())
	rootCmd.AddCommand(newRestartCmd())
	rootCmd.AddCommand(newOpenCmd())
	rootCmd.AddCommand(newSecretCmd())
	rootCmd.AddCommand(newVolumeCmd())
	rootCmd.AddCommand(newTokenCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newCompletionCmd())

	return rootCmd
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
