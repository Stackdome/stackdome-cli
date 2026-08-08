package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"github.com/spf13/cobra"
)

func newOpenCmd() *cobra.Command {
	var flagStack string

	cmd := &cobra.Command{
		Use:   "open [resource]",
		Short: "Open a resource's public URL in the browser",
		Long: `Open a resource's public URL in the browser.

With -o json|yaml no browser is launched: the public URLs are printed to stdout
as {"target": ..., "urls": [...]}.`,
		Args: cobra.MaximumNArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
			if err != nil {
				return err
			}

			// Public URLs live on the release status, not on the stack resources.
			live, err := ctx.Client.GetStackLiveStatus(cmd.Context(), stack)
			if err != nil {
				return err
			}

			resources := stack.Spec.StackResources

			resourceFilter := ""
			if len(args) > 0 {
				resourceFilter = args[0]
			}

			type publicURL struct {
				Resource string `json:"resource" yaml:"resource"`
				URL      string `json:"url" yaml:"url"`
			}

			var urls []publicURL
			for _, res := range resources {
				if resourceFilter != "" && res.Name != resourceFilter {
					continue
				}
				status := output.ResourceStatus(live, res.Name)
				if status == nil {
					continue
				}
				for _, ing := range status.PublicIngress {
					if ing.Url != nil && *ing.Url != "" {
						urls = append(urls, publicURL{Resource: res.Name, URL: *ing.Url})
					}
				}
			}

			if resourceFilter != "" && len(urls) == 0 {
				found := false
				for _, res := range resources {
					if res.Name == resourceFilter {
						found = true
						break
					}
				}
				if !found {
					return clierrors.NotFoundError("Resource", resourceFilter)
				}
				return clierrors.Newf("No public URLs found for resource %q", resourceFilter)
			}

			if len(urls) == 0 {
				return clierrors.New("No public URLs found in this stack")
			}

			target := urls[0].URL

			// Structured mode is for scripts: report the URLs on stdout instead
			// of launching a browser.
			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(struct {
					Target string      `json:"target" yaml:"target"`
					URLs   []publicURL `json:"urls" yaml:"urls"`
				}{Target: target, URLs: urls})
			}

			if len(urls) > 1 {
				tbl := ctx.Formatter.NewTable("RESOURCE", "URL")
				for _, u := range urls {
					tbl.AddRow(u.Resource, u.URL)
				}
				tbl.Render()
				fmt.Fprintln(os.Stderr)
			}

			fmt.Fprintf(os.Stderr, "Opening %s...\n", target)
			return openBrowser(target)
		})),
	}

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")

	return cmd
}

func openBrowser(url string) error {
	if !strings.Contains(url, "://") {
		url = "http://" + url
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return clierrors.Newf("unsupported platform %q — open %s manually", runtime.GOOS, url)
	}
	return cmd.Start()
}
