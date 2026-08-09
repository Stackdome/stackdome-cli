package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Stackdome/stackdome-cli/internal/client"
	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/stackfile"
	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/Stackdome/stackdome/pkg/models"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newStackfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stackfile",
		Short: "Inspect and export canonical Stackfiles",
	}
	cmd.AddCommand(newStackfileSchemaCmd())
	cmd.AddCommand(newStackfileExportCmd())
	return cmd
}

func newStackfileSchemaCmd() *cobra.Command {
	var (
		format     string
		outputFile string
	)
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Print the canonical Stackfile JSON Schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "json" {
				return clierrors.ValidationError("get stackfile-schema only supports -o json")
			}
			if outputFile != "" {
				if err := os.WriteFile(outputFile, stackfile.SchemaJSON, 0o644); err != nil {
					return clierrors.Wrapf(err, "Failed to write Stackfile schema: %s", outputFile)
				}
				return nil
			}
			_, err := cmd.OutOrStdout().Write(stackfile.SchemaJSON)
			if err != nil {
				return clierrors.Wrap(err, "Failed to write Stackfile schema")
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout())
			return err
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "json", "Output format (json)")
	cmd.Flags().StringVar(&outputFile, "output-file", "", "Write the exact embedded JSON Schema to this file (requires -o json)")
	return cmd
}

func newStackfileExportCmd() *cobra.Command {
	var (
		format     string
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "export <stack>",
		Short: "Export a stack as canonical Stackfile content",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			if format != "yaml" && format != "json" {
				return clierrors.ValidationError("export stackfile supports -o yaml or -o json")
			}

			stackID, err := resolveStackRef(ctx, cmd, args[0])
			if err != nil {
				return err
			}
			stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
			if err != nil {
				return err
			}
			if err := validateExportSettings(stack); err != nil {
				return clierrors.Wrapf(err, "Cannot export stack as Stackfile: %s", err)
			}
			if err := restoreExportConnectionNames(cmd.Context(), ctx.Client, stack); err != nil {
				return clierrors.Wrap(err, "Cannot export stack as Stackfile").WithDetail(err.Error())
			}
			normalizeStackfileExportInput(stack)
			sf, err := stackfile.FromStack(stack)
			if err != nil {
				return stackfileConversionError(err)
			}

			content, err := marshalStackfile(sf, format)
			if err != nil {
				return clierrors.Wrap(err, "Failed to encode Stackfile")
			}
			if outputFile != "" {
				if err := os.WriteFile(outputFile, content, 0o644); err != nil {
					return clierrors.Wrapf(err, "Failed to write Stackfile: %s", outputFile)
				}
				return nil
			}
			_, err = cmd.OutOrStdout().Write(content)
			return err
		})),
	}
	cmd.Flags().StringVarP(&format, "output", "o", "yaml", "Output format (yaml, json)")
	cmd.Flags().StringVar(&outputFile, "output-file", "", "Write Stackfile content to this file")
	return cmd
}

func stackfileConversionError(err error) error {
	// The canonical exporter uses this prefix only for structural features the
	// Stackfile schema cannot express. Other conversion/validation errors may
	// contain environment values and must remain private.
	if strings.HasPrefix(err.Error(), "not expressible in a stackfile: ") {
		return clierrors.Wrapf(err, "Cannot export stack as Stackfile: %s", err)
	}
	return clierrors.Wrap(err, "Cannot export stack as Stackfile")
}

// normalizeStackfileExportInput removes API facts that are not independent
// desired state. RestartRequestTime is an imperative action, while volume-mount
// connections duplicate StackResource.VolumeMounts (the source used for actual
// deployment and for Stackfile output). Replaying the timestamp or rejecting a
// stale canvas edge would both make a valid UI-created stack non-editable.
func normalizeStackfileExportInput(stack *openapi.Stack) {
	for i := range stack.Spec.StackResources {
		stack.Spec.StackResources[i].LifecycleConfig = nil
	}

	connections := make([]openapi.StackConnection, 0, len(stack.Spec.Connections))
	for _, connection := range stack.Spec.Connections {
		if !normalizeVolumeMountConnection(stack, connection) {
			connections = append(connections, connection)
		}
	}
	stack.Spec.Connections = connections
}

func normalizeVolumeMountConnection(stack *openapi.Stack, connection openapi.StackConnection) bool {
	if connection.Kind != string(models.ConnectionKindVolumeMount) ||
		connection.From.Type != string(models.TopologyNodeTypeVolume) ||
		connection.To.Type != string(models.TopologyNodeTypeStackResource) ||
		connection.From.Name == nil || connection.To.Name == nil {
		return false
	}

	mountPath := ""
	if connection.Config != nil {
		config := connection.Config.VolumeMountConfig
		if config == nil || config.SubPath != nil || config.ReadOnly != nil {
			return false
		}
		mountPath = config.MountPath
	}

	volumeExists := false
	for _, volume := range stack.Spec.Volumes {
		if volume.Name == *connection.From.Name {
			volumeExists = true
			break
		}
	}
	if !volumeExists {
		return false
	}

	for i := range stack.Spec.StackResources {
		resource := &stack.Spec.StackResources[i]
		if resource.Name != *connection.To.Name {
			continue
		}
		for _, mount := range resource.VolumeMounts {
			if mount.SourceVolumeName == *connection.From.Name {
				return true
			}
		}
		if mountPath == "" {
			return false
		}
		resource.VolumeMounts = append(resource.VolumeMounts, openapi.VolumeMount{
			SourceVolumeName: *connection.From.Name,
			TargetPath:       mountPath,
		})
		return true
	}
	return false
}

func validateExportSettings(stack *openapi.Stack) error {
	releaseRetention := int32(models.DefaultReleaseRetentionLimit)
	minSuccessful := int32(models.DefaultMinSuccessfulReleases)
	if stack.Settings != nil {
		if configured := stack.Settings.ReleaseRetentionLimit; configured != nil && *configured > 0 {
			releaseRetention = *configured
		}
		if configured := stack.Settings.MinSuccessfulReleases; configured != nil && *configured > 0 {
			minSuccessful = *configured
		}
	}

	if releaseRetention == int32(models.DefaultReleaseRetentionLimit) && minSuccessful == int32(models.DefaultMinSuccessfulReleases) {
		return nil
	}
	return fmt.Errorf(
		"stack settings cannot be represented: release_retention_limit=%d (default %d), min_successful_releases=%d (default %d)",
		releaseRetention,
		models.DefaultReleaseRetentionLimit,
		minSuccessful,
		models.DefaultMinSuccessfulReleases,
	)
}

func restoreExportConnectionNames(ctx context.Context, c *client.Client, stack *openapi.Stack) error {
	var (
		secretNames   map[string]string
		postgresNames map[string]string
	)

	for i := range stack.Spec.Connections {
		from := &stack.Spec.Connections[i].From
		if from.Name != nil && *from.Name != "" {
			continue
		}

		switch from.Type {
		case string(models.TopologyNodeTypeSecret):
			if from.Id == nil || *from.Id == "" {
				return fmt.Errorf("secret connection ref has neither name nor ID")
			}
			if secretNames == nil {
				secrets, err := c.ListSecrets(ctx)
				if err != nil {
					return err
				}
				secretNames = make(map[string]string, len(secrets))
				for _, secret := range secrets {
					if secret.Id != nil && *secret.Id != "" {
						secretNames[*secret.Id] = secret.Name
					}
				}
			}
			name, ok := secretNames[*from.Id]
			if !ok || name == "" {
				return fmt.Errorf("secret ID %q could not be resolved to a name", *from.Id)
			}
			from.Name = &name

		case string(models.TopologyNodeTypePostgresAddon):
			if from.Id == nil || *from.Id == "" {
				return fmt.Errorf("postgres addon connection ref has neither name nor ID")
			}
			if postgresNames == nil {
				addons, err := c.ListPostgresAddons(ctx)
				if err != nil {
					return err
				}
				postgresNames = make(map[string]string, len(addons))
				for _, addon := range addons {
					if addon.Id != nil && *addon.Id != "" {
						postgresNames[*addon.Id] = addon.Name
					}
				}
			}
			name, ok := postgresNames[*from.Id]
			if !ok || name == "" {
				return fmt.Errorf("postgres addon ID %q could not be resolved to a name", *from.Id)
			}
			from.Name = &name
		}
	}

	return nil
}

func marshalStackfile(sf *stackfile.Stackfile, format string) ([]byte, error) {
	content, err := yaml.Marshal(sf)
	if err != nil || format != "json" {
		return content, err
	}

	var document any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, err
	}
	return json.MarshalIndent(document, "", "  ")
}
