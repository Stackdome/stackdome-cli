package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	flagLogLevel string
	flagNoColor  bool
	flagOutput   string
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "stackdome",
		Short: "CLI for the Stackdome platform",
		Long: `Deploy, manage, and monitor your applications on Stackdome.

Every command runs non-interactively: pass --yes to skip confirmations,
STACKDOME_TOKEN (and STACKDOME_URL) to authenticate without a config file, and
-o json|yaml for machine-readable output. A token scoped too narrowly to look up
projects can name its scope directly with STACKDOME_ORG and STACKDOME_PROJECT. In -o json/yaml mode stdout carries
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
			ctx.Formatter.Writer = cmd.OutOrStdout()
			cmdutil.SetContext(cmd, ctx)
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&flagLogLevel, "log-level", "warn", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "table", "Output format (table, json, yaml)")

	rootCmd.AddCommand(documentCommand(newVersionCmd(), operationDocs("version", "Print the CLI version", "Print build version, commit, date, and Go runtime metadata. Structured output is available with -o json or yaml.", "stackdome version -o json")))
	rootCmd.AddCommand(documentCommand(newLoginCmd(), operationDocs("login", "Authenticate with a Stackdome server", "Authenticate with an API token or email and password, then persist the resulting credentials unless environment variables control the session.", "stackdome login --url https://api.stackdome.example --token <token>")))
	rootCmd.AddCommand(documentCommand(newLogoutCmd(), operationDocs("logout", "Clear stored credentials", "Remove credentials stored for the active Stackdome context. Environment-supplied credentials are not modified.", "stackdome logout")))
	rootCmd.AddCommand(documentCommand(newSignupCmd(), operationDocs("signup", "Create a new Stackdome account", "Create an account and organization, authenticate the new session, and persist its credentials.", "stackdome signup --url https://api.stackdome.example --name 'Ada Lovelace' --email ada@example.com --org example")))
	rootCmd.AddCommand(documentCommand(newWhoamiCmd(), operationDocs("whoami", "Show the active identity and scope", "Show the authenticated user, organization, project, server, and authentication method without revealing credentials.", "stackdome whoami -o json")))
	rootCmd.AddCommand(newCtxCmd())
	rootCmd.AddCommand(newDeployCmd())
	rootCmd.AddCommand(newApplyCmd())
	rootCmd.AddCommand(documentCommand(newStatusCmd(), operationDocs("status [resource]", "Show stack and resource status", "Show live status for the selected or specified stack, optionally filtered to one runtime resource. Pass --watch to refresh continuously.", "stackdome status --watch\n  stackdome status web --stack demo")))
	rootCmd.AddCommand(documentCommand(newValidateCmd(), operationDocs("validate", "Validate a Stackfile", "Load and validate a Stackfile locally without authenticating, saving stack state, or creating a release.", "stackdome validate -f stackfile.yaml")))
	rootCmd.AddCommand(documentCommand(newLogsCmd(), operationDocs("logs [resource]", "Read or follow runtime and build logs", "Read logs for the selected stack, optionally filtered to one runtime resource. Use `logs build <build-id>` for build logs and --follow to stream.", "stackdome logs --follow\n  stackdome logs web --tail 200\n  stackdome logs build <build-id> --follow")))
	rootCmd.AddCommand(documentCommand(newRestartCmd(), operationDocs("restart <resource>", "Restart a stack resource", "Request a restart of one runtime resource in the selected or specified stack.", "stackdome restart web --stack demo")))
	rootCmd.AddCommand(documentCommand(newOpenCmd(), operationDocs("open [resource]", "Open a public resource URL", "Open the first public URL for the selected stack or runtime resource. Structured output prints URLs without launching a browser.", "stackdome open web\n  stackdome open --stack demo -o json")))
	rootCmd.AddCommand(documentCommand(newInitCmd(), operationDocs("init", "Scaffold a new Stackfile", "Create stackfile.yaml from an interactive scaffold or convert a supported Compose file. Existing files require --force to overwrite.", "stackdome init\n  stackdome init --file compose.yaml")))
	rootCmd.AddCommand(documentCommand(newDoctorCmd(), operationDocs("doctor", "Diagnose CLI connectivity and context", "Check local configuration, server connectivity, authentication, project scope, selected stack, and Stackfile readiness without mutating remote state.", "stackdome doctor -o json")))
	rootCmd.AddCommand(documentCommand(newAPICmd(), operationDocs("api PATH", "Send an authenticated API request", "Send an authenticated request to a Stackdome API path. Mutating methods prompt unless --yes is supplied; request and response bodies use JSON.", "stackdome api /api/v1/users/current\n  stackdome api /api/v1/example -X POST --data '{\"name\":\"demo\"}' --yes")))
	rootCmd.AddCommand(documentCommand(newCompletionCmd(), operationDocs("completion [bash|zsh|fish]", "Generate shell completion", "Generate a completion script for bash, zsh, or fish and write it to stdout for installation by the shell.", "stackdome completion zsh > ~/.zfunc/_stackdome")))
	rootCmd.AddCommand(newCommandTree()...)
	configureRootHelpGroups(rootCmd)

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
			// Human-readable modes can pair the validation error with actionable
			// help. JSON stderr must remain exactly one error document.
			if len(args) == 0 && !c.HasSubCommands() && !commandUsesJSONOutput(c) {
				printHelpToStderr(c)
			}
			return clierrors.ValidationError(err.Error())
		}
		return nil
	}
}

func commandUsesJSONOutput(cmd *cobra.Command) bool {
	format, err := cmd.Flags().GetString("output")
	return err == nil && format == string(output.FormatJSON)
}

func printHelpToStderr(cmd *cobra.Command) {
	stdout := cmd.OutOrStdout()
	cmd.SetOut(cmd.ErrOrStderr())
	defer cmd.SetOut(stdout)
	_ = cmd.Help()
}

func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return runWithContext(ctx, os.Args[1:], os.Stdout, os.Stderr)
}

func runWithWriters(args []string, stdout, stderr io.Writer) int {
	return runWithContext(context.Background(), args, stdout, stderr)
}

func runWithContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	jsonErrors := requestsJSONOutput(args)
	rootCmd := newRootCmd()
	rootCmd.SetArgs(args)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if strings.HasPrefix(err.Error(), "unknown command ") {
			err = clierrors.ValidationError(err.Error())
		}
		msg := clierrors.UserMessage(err)
		exitCode := clierrors.ExitCodeFrom(err)
		if jsonErrors {
			_ = json.NewEncoder(stderr).Encode(struct {
				Error    string `json:"error"`
				ExitCode int    `json:"exit_code"`
			}{Error: msg, ExitCode: exitCode})
		} else {
			fmt.Fprintf(stderr, "Error: %s\n", msg)
		}
		return exitCode
	}
	return 0
}

func requestsJSONOutput(args []string) bool {
	jsonOutput := false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-o" || args[i] == "--output":
			if i+1 < len(args) {
				jsonOutput = args[i+1] == string(output.FormatJSON)
				i++
			}
		case strings.HasPrefix(args[i], "-o="):
			jsonOutput = strings.TrimPrefix(args[i], "-o=") == string(output.FormatJSON)
		case strings.HasPrefix(args[i], "--output="):
			jsonOutput = strings.TrimPrefix(args[i], "--output=") == string(output.FormatJSON)
		case len(args[i]) > len("-o") && strings.HasPrefix(args[i], "-o"):
			jsonOutput = strings.TrimPrefix(args[i], "-o") == string(output.FormatJSON)
		}
	}
	return jsonOutput
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
