package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
)

func newAddonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addon",
		Short: "Manage addons",
	}
	cmd.AddCommand(newPostgresCmd())
	return cmd
}

func newPostgresCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "postgres",
		Short: "Manage PostgreSQL addons",
	}

	cmd.AddCommand(newPostgresCreateCmd())
	cmd.AddCommand(newPostgresListCmd())
	cmd.AddCommand(newPostgresInfoCmd())
	cmd.AddCommand(newPostgresDeleteCmd())
	cmd.AddCommand(newPostgresCredentialsCmd())
	cmd.AddCommand(newPostgresBackupCmd())
	cmd.AddCommand(newPostgresBackupsCmd())
	return cmd
}

func newPostgresCreateCmd() *cobra.Command {
	var (
		flagDatabase  string
		flagSuperuser bool
		flagVersion   int32
		flagInstances int32
		flagStorage   string
		flagWait      bool
		flagTimeout   time.Duration
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a PostgreSQL addon",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			name := args[0]
			database := flagDatabase
			if database == "" {
				database = name
			}

			addon := openapi.PostgresAddon{
				Name: name,
				Spec: openapi.PostgresAddonSpec{
					Version:   openapi.PostgresVersion{Major: flagVersion},
					Instances: openapi.PostgresInstances{Count: flagInstances},
					Storage:   openapi.PostgresStorage{Size: flagStorage},
					Databases: []openapi.PostgresDatabase{{Name: database}},
				},
			}
			if flagSuperuser {
				addon.Spec.Configuration = &openapi.PostgresConfiguration{EnableSuperuserAccess: &flagSuperuser}
			}

			created, err := ctx.Client.CreatePostgresAddon(cmd.Context(), addon)
			if err != nil {
				return err
			}
			if flagWait {
				if created.Id == nil || *created.Id == "" {
					return clierrors.New("Created postgres addon response had no ID; cannot wait for readiness.")
				}
				waitCtx, cancel := waitContext(cmd.Context(), flagTimeout)
				defer cancel()
				waitCmd := *cmd
				waitCmd.SetContext(waitCtx)
				created, err = waitForPostgresAddon(ctx, &waitCmd, *created.Id, created.Name)
				if err := postgresWaitCommandError(cmd.Context(), waitCtx, createdName(addon, created), err); err != nil {
					return err
				}
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(created)
			}

			if flagWait {
				fmt.Fprintf(os.Stderr, "Postgres addon %q is %s.\n", created.Name, created.Status.GetState())
			} else {
				fmt.Fprintf(os.Stderr, "Postgres addon %q created.\n", created.Name)
			}
			return nil
		})),
	}

	cmd.Flags().StringVar(&flagDatabase, "database", "", "Initial database name (defaults to the addon name)")
	cmd.Flags().BoolVar(&flagSuperuser, "superuser", false, "Enable superuser access")
	cmd.Flags().Int32Var(&flagVersion, "version", 16, "PostgreSQL major version (13-17)")
	cmd.Flags().Int32Var(&flagInstances, "instances", 1, "Number of instances (1-5)")
	cmd.Flags().StringVar(&flagStorage, "storage", "10Gi", "Storage size")
	cmd.Flags().BoolVarP(&flagWait, "wait", "w", false, "Wait for the PostgreSQL addon to become ready")
	cmd.Flags().DurationVar(&flagTimeout, "timeout", defaultWaitTimeout, "Maximum time to wait for the PostgreSQL addon")
	return cmd
}

const postgresWaitPollInterval = 2 * time.Second

func waitForPostgresAddon(ctx *cmdutil.CommandContext, cmd *cobra.Command, addonID, name string) (*openapi.PostgresAddon, error) {
	ticker := time.NewTicker(postgresWaitPollInterval)
	defer ticker.Stop()

	for {
		addon, err := ctx.Client.GetPostgresAddon(cmd.Context(), addonID)
		if err != nil {
			return nil, err
		}
		state := ""
		message := ""
		if addon.Status != nil {
			state = addon.Status.GetState()
			message = addon.Status.GetMessage()
		}
		switch state {
		case "Ready", "Running":
			return addon, nil
		case "Failed", "Error":
			if message != "" {
				return nil, clierrors.Newf("Postgres addon %q entered terminal state %s: %s", name, state, message)
			}
			return nil, clierrors.Newf("Postgres addon %q entered terminal state %s", name, state)
		}

		select {
		case <-cmd.Context().Done():
			return nil, cmd.Context().Err()
		case <-ticker.C:
		}
	}
}

func postgresWaitCommandError(parent, wait context.Context, name string, err error) error {
	if parent.Err() != nil {
		return clierrors.ErrUserCanceled
	}
	if wait.Err() == context.DeadlineExceeded {
		return clierrors.Newf("Timed out waiting for postgres addon %q to become ready.", name).WithCode("TIMEOUT")
	}
	return err
}

func createdName(request openapi.PostgresAddon, response *openapi.PostgresAddon) string {
	if response != nil && response.Name != "" {
		return response.Name
	}
	return request.Name
}

func newPostgresListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List PostgreSQL addons",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			addons, err := ctx.Client.ListPostgresAddons(cmd.Context())
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(addons)
			}

			if len(addons) == 0 {
				fmt.Fprintln(os.Stderr, "No postgres addons found.")
				return nil
			}

			tbl := ctx.Formatter.NewTable("NAME", "VERSION", "INSTANCES", "STORAGE", "STATE", "CREATED")
			for _, a := range addons {
				tbl.AddRow(
					a.Name,
					fmt.Sprintf("%d", a.Spec.Version.Major),
					fmt.Sprintf("%d", a.Spec.Instances.Count),
					a.Spec.Storage.Size,
					addonState(a),
					addonCreated(a),
				)
			}
			tbl.Render()
			return nil
		})),
	}
}

func newPostgresInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show PostgreSQL addon details",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			addon, err := findPostgresAddon(ctx, cmd, args[0])
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(addon)
			}

			fmt.Printf("Name:      %s\n", addon.Name)
			fmt.Printf("Version:   %d\n", addon.Spec.Version.Major)
			fmt.Printf("Instances: %d\n", addon.Spec.Instances.Count)
			fmt.Printf("Storage:   %s\n", addon.Spec.Storage.Size)
			fmt.Printf("State:     %s\n", addonState(*addon))

			if addon.Status != nil && addon.Status.ConnectionInfo != nil {
				info := addon.Status.ConnectionInfo
				if info.Host != nil {
					fmt.Printf("Host:      %s\n", *info.Host)
				}
				if info.Port != nil {
					fmt.Printf("Port:      %d\n", *info.Port)
				}
				if len(info.Databases) > 0 {
					fmt.Println("\nDatabases:")
					for _, db := range info.Databases {
						fmt.Printf("  %s\n", db.GetName())
					}
				}
			}
			return nil
		})),
	}
}

func newPostgresDeleteCmd() *cobra.Command {
	var flagYes bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a PostgreSQL addon",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			addon, err := findPostgresAddon(ctx, cmd, args[0])
			if err != nil {
				return err
			}

			if _, err := cmdutil.Confirm(ctx.Formatter, fmt.Sprintf("Delete postgres addon %q and all its data?", args[0]), flagYes); err != nil {
				return err
			}

			if err := ctx.Client.DeletePostgresAddon(cmd.Context(), *addon.Id); err != nil {
				return err
			}

			return printMutationResult(ctx, mutationResult{
				Status:   "deleted",
				Resource: "postgres_addon",
				Name:     args[0],
				ID:       addon.GetId(),
			}, fmt.Sprintf("Postgres addon %q deleted.", args[0]))
		})),
	}

	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation")
	return cmd
}

func newPostgresCredentialsCmd() *cobra.Command {
	var flagSuperuser bool

	cmd := &cobra.Command{
		Use:   "credentials <name> <database>",
		Short: "Get just-in-time credentials for a database",
		Args:  cobra.ExactArgs(2),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			addon, err := findPostgresAddon(ctx, cmd, args[0])
			if err != nil {
				return err
			}

			creds, err := ctx.Client.GetPostgresCredentials(cmd.Context(), *addon.Id, args[1], flagSuperuser)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(creds)
			}

			for _, field := range []struct {
				label string
				value *string
			}{
				{"Host", creds.Host},
				{"Database", creds.Database},
				{"Username", creds.Username},
				{"Password", creds.Password},
				{"SSL Mode", creds.SslMode},
				{"URL", creds.ConnectionString},
			} {
				if field.value != nil && *field.value != "" {
					fmt.Printf("%s: %s\n", field.label, *field.value)
				}
			}
			if creds.Port != nil {
				fmt.Printf("Port: %d\n", *creds.Port)
			}
			return nil
		})),
	}

	cmd.Flags().BoolVar(&flagSuperuser, "superuser", false, "Request superuser credentials")
	return cmd
}

func newPostgresBackupCmd() *cobra.Command {
	var flagDescription string

	cmd := &cobra.Command{
		Use:   "backup <name>",
		Short: "Trigger an immediate backup",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			addon, err := findPostgresAddon(ctx, cmd, args[0])
			if err != nil {
				return err
			}

			resp, err := ctx.Client.BackupPostgresAddon(cmd.Context(), *addon.Id, flagDescription)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(resp)
			}

			fmt.Fprintf(os.Stderr, "Backup triggered for %q (%s).\n", args[0], resp.GetBackupId())
			return nil
		})),
	}

	cmd.Flags().StringVar(&flagDescription, "description", "", "Description for this backup")
	return cmd
}

func newPostgresBackupsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backups <name>",
		Short: "List backups of a PostgreSQL addon",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			addon, err := findPostgresAddon(ctx, cmd, args[0])
			if err != nil {
				return err
			}

			backups, err := ctx.Client.ListPostgresBackups(cmd.Context(), *addon.Id)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(backups)
			}

			if len(backups) == 0 {
				fmt.Fprintln(os.Stderr, "No backups found.")
				return nil
			}

			tbl := ctx.Formatter.NewTable("ID", "NAME", "TYPE", "PHASE", "STARTED")
			for _, b := range backups {
				started := "-"
				if b.StartedAt != nil {
					started = output.TimeAgo(*b.StartedAt)
				}
				tbl.AddRow(shortID(b.GetId()), b.GetName(), b.GetType(), b.GetPhase(), started)
			}
			tbl.Render()
			return nil
		})),
	}
}

func findPostgresAddon(ctx *cmdutil.CommandContext, cmd *cobra.Command, name string) (*openapi.PostgresAddon, error) {
	addon, err := ctx.Client.FindPostgresAddonByName(cmd.Context(), name)
	if err != nil {
		return nil, err
	}
	if addon == nil || addon.Id == nil {
		return nil, clierrors.NotFoundError("Postgres addon", name)
	}
	return addon, nil
}

func addonState(a openapi.PostgresAddon) string {
	if a.Status == nil || a.Status.State == nil {
		return "Unknown"
	}
	state := *a.Status.State
	switch state {
	case "Ready", "Running":
		return output.Green(state)
	case "Failed", "Error":
		return output.Red(state)
	case "Pending", "Provisioning":
		return output.Yellow(state)
	default:
		return state
	}
}

func addonCreated(a openapi.PostgresAddon) string {
	if a.CreatedAt == nil {
		return "-"
	}
	return output.TimeAgo(*a.CreatedAt)
}
