package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	openapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

func RenderStackStatus(w io.Writer, stack *openapi.Stack, showConditions bool) {
	state := "Unknown"
	if stack.Status != nil && stack.Status.State != nil {
		state = *stack.Status.State
	}

	fmt.Fprintf(w, "Stack: %-20s State: %s\n\n", Bold(stack.Name), StateColor(state))

	if stack.Status != nil && stack.Status.Message != nil && *stack.Status.Message != "" {
		fmt.Fprintf(w, "  %s\n\n", *stack.Status.Message)
	}

	renderResourceTable(w, stack.Spec.StackResources)

	renderFailures(w, stack.Spec.StackResources, showConditions)
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true)
	cellStyle   = lipgloss.NewStyle().PaddingRight(2)
)

func renderResourceTable(w io.Writer, resources []openapi.StackResource) {
	var rows [][]string
	for _, res := range resources {
		rows = append(rows, []string{
			res.Name,
			StateColor(resourceState(&res)),
			formatPorts(res.Ports),
			formatURL(&res),
		})
	}

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).BorderHeader(false).
		Headers("RESOURCE", "STATE", "PORTS", "URL").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	fmt.Fprintln(w, t)
}

func renderFailures(w io.Writer, resources []openapi.StackResource, showConditions bool) {
	hasFailures := false
	for _, res := range resources {
		if res.Status == nil || res.Status.LastFailure == nil {
			if showConditions && res.Status != nil && len(res.Status.Conditions) > 0 {
				if !hasFailures {
					hasFailures = true
				}
			}
			continue
		}
		if !hasFailures {
			fmt.Fprintf(w, "%s\n\n", Bold("FAILURES:"))
			hasFailures = true
		}
		renderResourceFailure(w, &res)
	}

	if showConditions {
		for _, res := range resources {
			if res.Status != nil && len(res.Status.Conditions) > 0 {
				renderConditions(w, &res)
			}
		}
	}
}

func renderResourceFailure(w io.Writer, res *openapi.StackResource) {
	failure := res.Status.LastFailure
	failureType := ""
	if failure.Type != nil {
		failureType = *failure.Type
	}

	fmt.Fprintf(w, "  %s — %s\n", Bold(res.Name), Red(failureType))

	if failure.Container != nil {
		renderContainerFailure(w, "Container", failure.Container)
	}
	if failure.InitContainer != nil {
		renderContainerFailure(w, "Init Container", failure.InitContainer)
	}
	if failure.Build != nil {
		renderBuildFailure(w, failure.Build)
	}

	if len(res.Status.Conditions) > 0 {
		renderLastConditions(w, res.Status.Conditions, 3)
	}

	fmt.Fprintln(w)
}

func renderContainerFailure(w io.Writer, label string, detail *openapi.ContainerFailureDetail) {
	parts := []string{}
	if detail.FailureType != nil {
		parts = append(parts, string(*detail.FailureType))
	}
	if detail.ExitCode != nil {
		parts = append(parts, fmt.Sprintf("exit code %d", *detail.ExitCode))
	}
	if detail.RestartCount != nil && *detail.RestartCount > 0 {
		parts = append(parts, fmt.Sprintf("restarted %d times", *detail.RestartCount))
	}
	fmt.Fprintf(w, "    %s: %s\n", label, strings.Join(parts, ", "))

	if detail.Message != nil && *detail.Message != "" {
		fmt.Fprintf(w, "    Message: %q\n", *detail.Message)
	}
}

func renderBuildFailure(w io.Writer, detail *openapi.BuildFailureDetail) {
	parts := []string{}
	if detail.FailureType != nil {
		parts = append(parts, string(*detail.FailureType))
	}
	if detail.ExitCode != nil {
		parts = append(parts, fmt.Sprintf("exit code %d", *detail.ExitCode))
	}
	fmt.Fprintf(w, "    Build: %s\n", strings.Join(parts, ", "))

	if detail.Message != nil && *detail.Message != "" {
		fmt.Fprintf(w, "    Message: %q\n", *detail.Message)
	}
}

func renderLastConditions(w io.Writer, conditions []openapi.Condition, max int) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %s\n", Dim("Last conditions:"))

	count := len(conditions)
	start := 0
	if count > max {
		start = count - max
	}

	var rows [][]string
	for _, c := range conditions[start:] {
		rows = append(rows, conditionRow(c))
	}

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).BorderHeader(false).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			return cellStyle.Padding(0, 1, 0, 4)
		})

	fmt.Fprintln(w, t)
}

func conditionRow(c openapi.Condition) []string {
	status, condType, reason, message, age := "", "", "", "", ""
	if c.Status != nil {
		status = *c.Status
	}
	if c.Type != nil {
		condType = *c.Type
	}
	if c.Reason != nil {
		reason = *c.Reason
	}
	if c.Message != nil {
		message = *c.Message
	}
	if c.LastTransitionTime != nil {
		age = Dim(timeAgo(*c.LastTransitionTime))
	}
	return []string{status, condType, reason, message, age}
}

func renderConditions(w io.Writer, res *openapi.StackResource) {
	fmt.Fprintf(w, "\n%s conditions:\n", Bold(res.Name))

	var rows [][]string
	for _, c := range res.Status.Conditions {
		rows = append(rows, conditionRow(c))
	}

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).BorderHeader(false).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			return cellStyle
		})

	fmt.Fprintln(w, t)
}

func resourceState(res *openapi.StackResource) string {
	if res.Status == nil || res.Status.State == nil {
		return "Unknown"
	}
	return *res.Status.State
}

func formatPorts(ports []openapi.Port) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		proto := "HTTP"
		if p.Protocol != nil {
			proto = *p.Protocol
		}
		s := fmt.Sprintf("%d/%s", p.Number, proto)
		if p.ExposedToPublic {
			s += " (public)"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func formatURL(res *openapi.StackResource) string {
	if res.Status == nil || len(res.Status.PublicIngress) == 0 {
		return "-"
	}
	urls := make([]string, 0)
	for _, ing := range res.Status.PublicIngress {
		if ing.Url != nil && *ing.Url != "" {
			urls = append(urls, *ing.Url)
		}
	}
	if len(urls) == 0 {
		return "-"
	}
	return strings.Join(urls, ", ")
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
