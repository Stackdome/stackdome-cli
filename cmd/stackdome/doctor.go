package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
)

const (
	doctorStatusOK      = "ok"
	doctorStatusFail    = "failed"
	doctorStatusSkipped = "skipped"
	doctorStatusUnknown = "unknown"
)

type doctorCheck struct {
	Status string `json:"status" yaml:"status"`
	Error  string `json:"error,omitempty" yaml:"error,omitempty"`
}

type doctorCLI struct {
	doctorCheck
	Version string `json:"version" yaml:"version"`
	Commit  string `json:"commit" yaml:"commit"`
	Built   string `json:"built" yaml:"built"`
	Go      string `json:"go" yaml:"go"`
	OSArch  string `json:"os_arch" yaml:"os_arch"`
}

type doctorServer struct {
	doctorCheck
	URL       string `json:"url" yaml:"url"`
	Reachable bool   `json:"reachable" yaml:"reachable"`
}

type doctorAuth struct {
	doctorCheck
	Configured    bool   `json:"configured" yaml:"configured"`
	Authenticated bool   `json:"authenticated" yaml:"authenticated"`
	User          string `json:"user,omitempty" yaml:"user,omitempty"`
}

type doctorOrganization struct {
	doctorCheck
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
}

type doctorProject struct {
	doctorCheck
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Implicit bool   `json:"implicit" yaml:"implicit"`
}

type doctorStack struct {
	doctorCheck
	Configured bool   `json:"configured" yaml:"configured"`
	ID         string `json:"id,omitempty" yaml:"id,omitempty"`
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
}

type doctorCompatibility struct {
	doctorCheck
	Detail string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

// doctorResult deliberately does not depend on a Hub metadata endpoint. The
// alpha server has no stable metadata contract yet, so compatibility remains
// explicit rather than guessing from a server response.
type doctorResult struct {
	CLI           doctorCLI           `json:"cli" yaml:"cli"`
	Server        doctorServer        `json:"server" yaml:"server"`
	Auth          doctorAuth          `json:"auth" yaml:"auth"`
	Organization  doctorOrganization  `json:"organization" yaml:"organization"`
	Project       doctorProject       `json:"project" yaml:"project"`
	Stack         doctorStack         `json:"stack" yaml:"stack"`
	Compatibility doctorCompatibility `json:"compatibility" yaml:"compatibility"`
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check CLI, connection, authentication, and current context",
		RunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, _ []string) error {
			result, failed := runDoctorChecks(cmd.Context(), ctx)
			if !ctx.Formatter.IsTable() {
				if err := ctx.Formatter.PrintStructured(result); err != nil {
					return err
				}
			} else {
				renderDoctorTable(ctx, result)
			}
			if failed {
				return clierrors.New("One or more doctor checks failed.")
			}
			return nil
		}),
	}
}

func runDoctorChecks(callCtx context.Context, ctx *cmdutil.CommandContext) (doctorResult, bool) {
	cfg := ctx.Config
	result := doctorResult{
		CLI:    doctorCLI{doctorCheck: doctorCheck{Status: doctorStatusOK}, Version: Version, Commit: GitCommit, Built: BuildDate, Go: runtime.Version(), OSArch: runtime.GOOS + "/" + runtime.GOARCH},
		Server: doctorServer{URL: cfg.ServerURL},
		Auth:   doctorAuth{Configured: cfg.AccessToken != ""},
		Compatibility: doctorCompatibility{
			doctorCheck: doctorCheck{Status: doctorStatusUnknown},
			Detail:      "server metadata endpoint is unavailable in this CLI version",
		},
	}

	result.Server.Reachable, result.Server.Error = doctorReachable(callCtx, cfg)
	if result.Server.Reachable {
		result.Server.Status = doctorStatusOK
	} else {
		result.Server.Status = doctorStatusFail
	}

	orgID := cfg.OrganizationID
	projectName := cfg.ProjectName
	authenticatedOrgID := ""
	var scopedStack *openapi.Stack
	scopedValidated := false
	if ctx.Client == nil {
		result.Auth.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: "no credentials configured"}
	} else {
		user, err := ctx.Client.GetCurrentUser(callCtx)
		if err != nil {
			if doctorDiscoveryForbidden(err) && orgID != "" && projectName != "" {
				stack, scopeErr := validateConfiguredDoctorScope(callCtx, ctx, cfg)
				if scopeErr == nil {
					scopedValidated = true
					scopedStack = stack
					result.Auth.doctorCheck = doctorCheck{Status: doctorStatusOK}
					result.Auth.Authenticated = true
					result.Organization.doctorCheck = doctorCheck{Status: doctorStatusOK}
					result.Organization.ID = orgID
					result.Project.doctorCheck = doctorCheck{Status: doctorStatusOK}
					result.Project.Name = projectName
					result.Project.Implicit = true
				} else {
					result.Auth.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: doctorScopeFailure(err, scopeErr)}
				}
			} else {
				result.Auth.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: clierrors.UserMessage(err)}
			}
		} else {
			result.Auth.doctorCheck = doctorCheck{Status: doctorStatusOK}
			result.Auth.Authenticated = true
			result.Auth.User = userDisplayName(user)
			authenticatedOrgID = user.GetOrganisationId()
			if orgID == "" {
				orgID = authenticatedOrgID
			}
		}
	}

	switch {
	case scopedValidated:
		// The scoped endpoint accepted this exact organization/project pair.
		result.Organization.doctorCheck = doctorCheck{Status: doctorStatusOK}
		result.Organization.ID = orgID
	case !result.Auth.Authenticated && orgID != "":
		result.Organization.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: "organization could not be validated without authentication"}
		result.Organization.ID = orgID
	case orgID == "":
		result.Organization.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: "organization is not configured"}
	case authenticatedOrgID == "":
		result.Organization.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: "authenticated user has no organization"}
		result.Organization.ID = orgID
	case orgID != authenticatedOrgID:
		result.Organization.doctorCheck = doctorCheck{
			Status: doctorStatusFail,
			Error:  fmt.Sprintf("configured organization %q does not match authenticated organization %q", orgID, authenticatedOrgID),
		}
		result.Organization.ID = orgID
	default:
		result.Organization.doctorCheck = doctorCheck{Status: doctorStatusOK}
		result.Organization.ID = orgID
	}

	if ctx.Client != nil && result.Auth.Authenticated && authenticatedOrgID != "" {
		name, err := ctx.Client.ResolveDefaultProject(callCtx, authenticatedOrgID)
		if err != nil {
			if doctorDiscoveryForbidden(err) && orgID != "" && projectName != "" {
				stack, scopeErr := validateConfiguredDoctorScope(callCtx, ctx, cfg)
				if scopeErr == nil {
					scopedValidated = true
					scopedStack = stack
					result.Project.doctorCheck = doctorCheck{Status: doctorStatusOK}
					result.Project.Name = projectName
					result.Project.Implicit = true
				} else {
					result.Project.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: doctorScopeFailure(err, scopeErr)}
					result.Project.Name = projectName
				}
			} else {
				result.Project.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: clierrors.UserMessage(err)}
				result.Project.Name = projectName
			}
		} else if projectName != "" && projectName != name {
			result.Project.doctorCheck = doctorCheck{
				Status: doctorStatusFail,
				Error:  fmt.Sprintf("configured project %q does not match signup default %q", projectName, name),
			}
			result.Project.Name = projectName
		} else {
			projectName = name
			result.Project.doctorCheck = doctorCheck{Status: doctorStatusOK}
			result.Project.Name = projectName
			result.Project.Implicit = true
		}
	}
	if result.Project.Status == "" {
		if !result.Auth.Authenticated && projectName != "" {
			result.Project.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: "default project could not be validated without authentication"}
			result.Project.Name = projectName
		} else if projectName == "" {
			result.Project.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: "default project could not be determined"}
		} else {
			result.Project.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: "default project could not be validated"}
			result.Project.Name = projectName
		}
	}

	result.Stack.Configured = cfg.CurrentStack != ""
	if !result.Stack.Configured {
		result.Stack.doctorCheck = doctorCheck{Status: doctorStatusSkipped}
	} else if scopedValidated && scopedStack != nil {
		result.Stack.doctorCheck = doctorCheck{Status: doctorStatusOK}
		result.Stack.ID = scopedStack.GetId()
		result.Stack.Name = scopedStack.GetName()
	} else if ctx.Client == nil || !result.Auth.Authenticated || result.Organization.Status != doctorStatusOK || result.Project.Status != doctorStatusOK {
		result.Stack.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: "current stack could not be checked without an authenticated project scope"}
		result.Stack.ID = cfg.CurrentStack
	} else {
		ctx.Client.SetOrgAndProject(orgID, projectName)
		stackID := cfg.CurrentStack
		if !looksLikeUUID(stackID) {
			lookupCmd := &cobra.Command{}
			lookupCmd.SetContext(callCtx)
			id, err := resolveStackRef(ctx, lookupCmd, stackID)
			if err != nil {
				result.Stack.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: clierrors.UserMessage(err)}
			} else {
				stackID = id
			}
		}
		if result.Stack.Status == "" {
			stack, err := ctx.Client.GetStack(callCtx, stackID)
			if err != nil {
				result.Stack.doctorCheck = doctorCheck{Status: doctorStatusFail, Error: clierrors.UserMessage(err)}
			} else {
				result.Stack.doctorCheck = doctorCheck{Status: doctorStatusOK}
				result.Stack.ID = stack.GetId()
				result.Stack.Name = stack.GetName()
			}
		}
	}

	return result, doctorFailed(result)
}

func doctorDiscoveryForbidden(err error) bool {
	var cliErr *clierrors.CLIError
	return errors.As(err, &cliErr) && cliErr.Code == "FORBIDDEN"
}

// validateConfiguredDoctorScope mirrors RequireAuth's known-scope behavior:
// scoped tokens may be unable to discover users/projects but can still prove
// the configured scope by making a request inside that project.
func validateConfiguredDoctorScope(callCtx context.Context, ctx *cmdutil.CommandContext, cfg *config.Config) (*openapi.Stack, error) {
	ctx.Client.SetOrgAndProject(cfg.OrganizationID, cfg.ProjectName)
	if cfg.CurrentStack == "" {
		_, err := ctx.Client.ListStacks(callCtx)
		return nil, err
	}
	if looksLikeUUID(cfg.CurrentStack) {
		return ctx.Client.GetStack(callCtx, cfg.CurrentStack)
	}

	stacks, err := ctx.Client.ListStacks(callCtx)
	if err != nil {
		return nil, err
	}
	for i := range stacks {
		if stacks[i].Name == cfg.CurrentStack || stacks[i].GetId() == cfg.CurrentStack {
			return &stacks[i], nil
		}
	}
	return nil, clierrors.NotFoundError("Stack", cfg.CurrentStack)
}

func doctorScopeFailure(discoveryErr, scopeErr error) string {
	return fmt.Sprintf("%s; configured scope check failed: %s", discoveryErr, scopeErr)
}

func doctorReachable(ctx context.Context, cfg *config.Config) (bool, string) {
	if cfg.ServerURL == "" {
		return false, "server URL is not configured"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	if cfg.Insecure {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.ServerURL, nil)
	if err != nil {
		return false, err.Error()
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	resp.Body.Close()
	return true, ""
}

func doctorFailed(result doctorResult) bool {
	return result.CLI.Status == doctorStatusFail ||
		result.Server.Status == doctorStatusFail ||
		result.Auth.Status == doctorStatusFail ||
		result.Organization.Status == doctorStatusFail ||
		result.Project.Status == doctorStatusFail ||
		(result.Stack.Configured && result.Stack.Status == doctorStatusFail)
}

func renderDoctorTable(ctx *cmdutil.CommandContext, result doctorResult) {
	table := ctx.Formatter.NewTable("CHECK", "STATUS", "DETAIL")
	table.AddRow("CLI", result.CLI.Status, doctorDetail(result.CLI.doctorCheck, result.CLI.Version))
	table.AddRow("Server", result.Server.Status, doctorDetail(result.Server.doctorCheck, result.Server.URL))
	table.AddRow("Authentication", result.Auth.Status, doctorDetail(result.Auth.doctorCheck, result.Auth.User))
	table.AddRow("Organization", result.Organization.Status, doctorDetail(result.Organization.doctorCheck, result.Organization.ID))
	table.AddRow("Default project", result.Project.Status, doctorDetail(result.Project.doctorCheck, result.Project.Name))
	if result.Stack.Configured {
		table.AddRow("Current stack", result.Stack.Status, doctorDetail(result.Stack.doctorCheck, fmt.Sprintf("%s %s", result.Stack.Name, result.Stack.ID)))
	}
	table.AddRow("Compatibility", result.Compatibility.Status, result.Compatibility.Detail)
	table.Render()
}

func doctorDetail(check doctorCheck, fallback string) string {
	if check.Error != "" {
		return check.Error
	}
	return fallback
}
