package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
	"github.com/stackdome/cli/internal/output"
	"github.com/stackdome/cli/internal/stackfile"
)

func newDeployCmd() *cobra.Command {
	var (
		flagFile string
		flagName string
		flagWait bool
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a stack from a stackfile or JSON",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stack, err := loadStack(flagFile, flagName)
			if err != nil {
				return err
			}

			existing, err := ctx.Client.FindStackByName(cmd.Context(), stack.Name)
			if err != nil {
				return err
			}

			var result *openapi.Stack
			if existing != nil {
				fmt.Fprintf(os.Stderr, "Updating stack %q...\n", stack.Name)
				result, err = ctx.Client.UpdateStack(cmd.Context(), *existing.Id, *stack)
			} else {
				fmt.Fprintf(os.Stderr, "Creating stack %q...\n", stack.Name)
				result, err = ctx.Client.CreateStack(cmd.Context(), *stack)
			}
			if err != nil {
				return err
			}

			if err := ctx.Config.SetCurrentStack(*result.Id); err != nil {
				return err
			}

			if flagWait {
				spin := output.NewSpinner("Waiting for stack to be ready...")
				spin.Start()
				final, err := waitForStack(ctx, cmd, *result.Id)
				spin.Stop()
				if err != nil {
					return err
				}

				if !ctx.Formatter.IsTable() {
					return ctx.Formatter.PrintStructured(final)
				}

				live, err := ctx.Client.GetStackLiveStatus(cmd.Context(), final)
				if err != nil {
					return err
				}
				output.RenderStackStatus(os.Stdout, final, live, false)
				return nil
			}

			fmt.Fprintf(os.Stderr, "\nStack %q submitted. Track progress with:\n", result.Name)
			fmt.Fprintf(os.Stderr, "  stackdome status          # current state\n")
			fmt.Fprintf(os.Stderr, "  stackdome status --watch  # live updates\n")
			fmt.Fprintf(os.Stderr, "  stackdome logs            # stream logs\n")
			return nil
		})),
	}

	cmd.Flags().StringVarP(&flagFile, "file", "f", "stackfile.yaml", "Path to stackfile or stack JSON")
	cmd.Flags().StringVar(&flagName, "name", "", "Override stack name")
	cmd.Flags().BoolVarP(&flagWait, "wait", "w", false, "Wait for stack to be ready")

	return cmd
}

func loadStack(path, nameOverride string) (*openapi.Stack, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		stack, err := stackfile.LoadJSON(path)
		if err != nil {
			return nil, err
		}
		if nameOverride != "" {
			stack.Name = nameOverride
		}
		return stack, nil

	case ".yaml", ".yml":
		sf, err := stackfile.Load(path)
		if err != nil {
			return nil, err
		}
		if err := stackfile.ResolveEnvFiles(sf, filepath.Dir(path)); err != nil {
			return nil, err
		}
		if nameOverride != "" {
			sf.Name = nameOverride
		}
		stack := sf.ToStack()
		return &stack, nil

	default:
		return nil, clierrors.ValidationError("Unsupported file format: " + ext)
	}
}

func waitForStack(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID string) (*openapi.Stack, error) {
	timeout := time.After(5 * time.Minute)
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-cmd.Context().Done():
			return nil, clierrors.New("Interrupted")
		case <-timeout:
			stack, _ := ctx.Client.GetStack(cmd.Context(), stackID)
			if stack != nil {
				return stack, nil
			}
			return nil, clierrors.New("Timed out waiting for stack to be ready")
		case <-tick.C:
			stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
			if err != nil {
				continue
			}
			rel := output.StackRelease(stack)
			if rel == nil || rel.State == nil {
				continue
			}
			state := string(*rel.State)
			switch state {
			case "Released":
				fmt.Fprintf(os.Stderr, "Stack is ready.\n")
				return stack, nil
			case "Failed", "Cancelled", "Superseded":
				fmt.Fprintf(os.Stderr, "Release %s.\n", strings.ToLower(state))
				return stack, nil
			}
		}
	}
}
