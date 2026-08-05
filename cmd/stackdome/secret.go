package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
	"github.com/stackdome/cli/internal/output"
)

func newSecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage secrets",
	}

	cmd.AddCommand(newSecretListCmd())
	cmd.AddCommand(newSecretInfoCmd())
	cmd.AddCommand(newSecretCreateCmd())
	cmd.AddCommand(newSecretSetCmd())
	cmd.AddCommand(newSecretDeleteCmd())
	return cmd
}

func newSecretListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all secrets",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			secrets, err := ctx.Client.ListSecrets(cmd.Context())
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(secrets)
			}

			if len(secrets) == 0 {
				fmt.Fprintln(os.Stderr, "No secrets found.")
				return nil
			}

			tbl := ctx.Formatter.NewTable("NAME", "TYPE", "KEYS", "CREATED")
			for _, s := range secrets {
				tbl.AddRow(
					s.Name,
					string(s.Type),
					secretKeys(s),
					secretCreated(s),
				)
			}
			tbl.Render()
			return nil
		})),
	}
}

func newSecretInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show secret details",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			secret, err := ctx.Client.FindSecretByName(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if secret == nil {
				return clierrors.NotFoundError("Secret", args[0])
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(secret)
			}

			fmt.Printf("Name:        %s\n", secret.Name)
			fmt.Printf("Type:        %s\n", secret.Type)
			if secret.Description != nil && *secret.Description != "" {
				fmt.Printf("Description: %s\n", *secret.Description)
			}

			if len(secret.Data) > 0 {
				fmt.Println("\nKeys:")
				for _, d := range secret.Data {
					fmt.Printf("  %s\n", d.Key)
				}
			}

			if secret.CreatedAt != nil {
				fmt.Printf("\nCreated:     %s\n", secret.CreatedAt.Format(time.DateTime))
			}
			if secret.UpdatedAt != nil {
				fmt.Printf("Updated:     %s\n", secret.UpdatedAt.Format(time.DateTime))
			}
			return nil
		})),
	}
}

func newSecretCreateCmd() *cobra.Command {
	var (
		flagData        []string
		flagFromFile    string
		flagType        string
		flagDescription string
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new secret",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			data, err := collectSecretData(flagData, flagFromFile)
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return clierrors.ValidationError("At least one --data or --from-file is required")
			}

			secret := openapi.Secret{
				Name: args[0],
				Type: openapi.SecretType(flagType),
				Data: data,
			}
			if flagDescription != "" {
				secret.Description = &flagDescription
			}

			result, err := ctx.Client.CreateSecret(cmd.Context(), secret)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(result)
			}

			fmt.Fprintf(os.Stderr, "Secret %q created.\n", result.Name)
			return nil
		})),
	}

	cmd.Flags().StringArrayVar(&flagData, "data", nil, "Secret data as KEY=VALUE (repeatable)")
	cmd.Flags().StringVar(&flagFromFile, "from-file", "", "Read KEY=VALUE pairs from file")
	cmd.Flags().StringVar(&flagType, "type", "Generic", "Secret type (Generic, DockerRegistry, GitCredentials, UsernamePassword, Token, SSHKey)")
	cmd.Flags().StringVar(&flagDescription, "description", "", "Secret description")

	return cmd
}

func newSecretSetCmd() *cobra.Command {
	var (
		flagData     []string
		flagFromFile string
	)

	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Update a secret's data",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			existing, err := ctx.Client.FindSecretByName(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if existing == nil {
				return clierrors.NotFoundError("Secret", args[0])
			}

			data, err := collectSecretData(flagData, flagFromFile)
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return clierrors.ValidationError("At least one --data or --from-file is required")
			}

			updated := openapi.Secret{
				Name: existing.Name,
				Type: existing.Type,
				Data: data,
			}
			if existing.Description != nil {
				updated.Description = existing.Description
			}

			result, err := ctx.Client.UpdateSecret(cmd.Context(), *existing.Id, updated)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(result)
			}

			fmt.Fprintf(os.Stderr, "Secret %q updated.\n", args[0])
			return nil
		})),
	}

	cmd.Flags().StringArrayVar(&flagData, "data", nil, "Secret data as KEY=VALUE (repeatable)")
	cmd.Flags().StringVar(&flagFromFile, "from-file", "", "Read KEY=VALUE pairs from file")

	return cmd
}

func newSecretDeleteCmd() *cobra.Command {
	var flagYes bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a secret",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			secret, err := ctx.Client.FindSecretByName(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if secret == nil {
				return clierrors.NotFoundError("Secret", args[0])
			}

			if _, err := cmdutil.Confirm(ctx.Formatter, fmt.Sprintf("Delete secret %q?", args[0]), flagYes); err != nil {
				return err
			}

			if err := ctx.Client.DeleteSecret(cmd.Context(), *secret.Id); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Secret %q deleted.\n", args[0])
			return nil
		})),
	}

	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation")
	return cmd
}

func collectSecretData(dataFlags []string, fromFile string) ([]openapi.SecretData, error) {
	var data []openapi.SecretData

	for _, kv := range dataFlags {
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, clierrors.ValidationError(fmt.Sprintf("invalid --data format %q, expected KEY=VALUE", kv))
		}
		data = append(data, openapi.SecretData{Key: key, Value: value})
	}

	if fromFile != "" {
		fileData, err := parseEnvFile(fromFile)
		if err != nil {
			return nil, err
		}
		data = append(data, fileData...)
	}

	return data, nil
}

func parseEnvFile(path string) ([]openapi.SecretData, error) {
	env, err := godotenv.Read(path)
	if err != nil {
		return nil, clierrors.Wrapf(err, "Failed to read file %q", path)
	}
	var data []openapi.SecretData
	for key, value := range env {
		data = append(data, openapi.SecretData{Key: key, Value: value})
	}
	return data, nil
}

func secretKeys(s openapi.Secret) string {
	if len(s.Data) == 0 {
		return "-"
	}
	keys := make([]string, len(s.Data))
	for i, d := range s.Data {
		keys[i] = d.Key
	}
	return strings.Join(keys, ", ")
}

func secretCreated(s openapi.Secret) string {
	if s.CreatedAt == nil {
		return "-"
	}
	return output.TimeAgo(*s.CreatedAt)
}
