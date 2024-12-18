package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/config"
)

const (
	green = "\033[32m" // Green color
	red   = "\033[31m" // Red color
	reset = "\033[0m"  // Reset to default color

	nameWidth          = 20
	stateWidth         = 10 // For "Ready", "Pending", "Error"
	readyWidth         = 8  // Include explicit padding for "✓" or "×"
	ingressWidth       = 40
	buildWidth         = 20
	stateColWidth      = 13
	restartStatusWidth = 22
)

func (w *workspaceHandler) Status(ctx context.Context, runtime *config.Runtime) error {
	currentWorkspaceName := runtime.Config().CurrentWorkspace
	if currentWorkspaceName == nil {
		return workspaceHandlerErr("current workspace not set")
	}

	_, serr := w.getCurretWorkspaceStorage(ctx, *currentWorkspaceName)
	if serr != nil {
		return workspaceHandlerErr("failed to get current workspace storage: %w", serr)
	}

	currentWorkspace, wgerr := w.workspaceService.GetWorkspaceByName(ctx, *currentWorkspaceName)
	if wgerr != nil {
		return workspaceHandlerErr("failed to get workspace: %w", wgerr)
	}

	currentBuilds, bErr := w.workspaceService.ListWorkspaceBuilds(ctx, currentWorkspace)
	if bErr != nil {
		return workspaceHandlerErr("failed to list workspace builds: %w", bErr)
	}

	if runtime.Args.IsAllResources() {
		PrintWorkspaceStatus(currentWorkspace, currentBuilds)
		return nil
	}
	printResource(currentWorkspace, currentBuilds, runtime.Args.GetResourceName())
	return nil
}

func printResource(workspace *v1alpha1.Workspace, builds []v1alpha1.ResourceBuild, resourceName string) {
	for _, res := range workspace.Resources {
		if res.Name == resourceName {
			fmt.Printf("Resource: %s\n", res.Name)
			fmt.Printf("Status: %s %s\n",
				getStatusIndicator(res.IsAvailable()),
				res.Status.State,
			)
			fmt.Println()

			printBuildsForResource(builds, res.Name)
			printIngressDetails(res.Status.PublicIngresses)
			return
		}
	}
	fmt.Printf("Resource '%s' not found\n", resourceName)
}

func (w *workspaceHandler) getCurretWorkspaceStorage(ctx context.Context, workspaceName string) (*v1alpha1.WorkspaceStorage, error) {
	storages, err := w.workspaceStorageService.GetCurrentWorkspaceStorages(ctx)
	if err != nil {
		return nil, workspaceHandlerErr("failed to get workspace storages: %w", err)
	}
	for _, storage := range storages {
		if storage.WorkspaceName == workspaceName {
			return storage, nil
		}
	}
	return nil, nil
}

func PrintWorkspaceStatus(w *v1alpha1.Workspace, builds []v1alpha1.ResourceBuild) {
	fmt.Printf("Workspace: %s\n", w.Name)
	fmt.Printf("Status: %s %s\n",
		getStatusIndicator(w.IsAvailable()),
		w.Status.State,
	)
	fmt.Printf("CreatedAt: %s\n", w.CreatedAt.Local())
	fmt.Println()

	totalWidth := nameWidth + stateWidth + readyWidth + ingressWidth + buildWidth + stateColWidth + restartStatusWidth

	fmt.Println("RESOURCES:")
	fmt.Printf("%-*s %-*s %-*s %-*s %-*s %-*s %s\n",
		nameWidth, "NAME",
		stateWidth, "STATE",
		readyWidth, "READY",
		ingressWidth, "INGRESS",
		buildWidth, "CURRENT_BUILD",
		stateColWidth, "BUILD_STATE",
		"RESTART STATUS",
	)
	fmt.Printf("%s\n", strings.Repeat("-", totalWidth))

	for _, res := range w.Resources {
		currentBuild := getCurrentBuild(builds, res)
		var buildState string
		var buildID string
		if currentBuild == nil {
			buildState = "-"
			buildID = "-"
		} else {
			buildState = currentBuild.Status.State
			buildID = currentBuild.ID
		}

		ingressLines := formatIngressDetails(res.Status.PublicIngresses)

		if len(ingressLines) == 0 {
			fmt.Printf("%-*s %-*s %-*s %-*s %-*s %-*s %s\n",
				nameWidth, res.Name,
				stateWidth, res.Status.State,
				readyWidth, getStatusIndicator(res.IsAvailable()),
				ingressWidth, "<none>",
				buildWidth, buildID,
				stateColWidth, buildState,
				formatRestartStatus(res.LifecycleConfig, res.Status),
			)
		} else {
			// Print first line with all columns
			fmt.Printf("%-*s %-*s %-*s %-*s %-*s %-*s %s\n",
				nameWidth, res.Name,
				stateWidth, res.Status.State,
				readyWidth, getStatusIndicator(res.IsAvailable()),
				ingressWidth, ingressLines[0],
				buildWidth, buildID,
				stateColWidth, buildState,
				formatRestartStatus(res.LifecycleConfig, res.Status),
			)

			// Print additional ingress lines aligned under the "INGRESS" column
			for _, ingressLine := range ingressLines[1:] {
				fmt.Printf("%-*s %-*s %-*s %-*s\n",
					nameWidth, "",
					stateWidth, "",
					readyWidth, "",
					ingressWidth, ingressLine,
				)
			}
		}
	}
}

func formatIngressDetails(ingresses []v1alpha1.Ingress) []string {
	if len(ingresses) == 0 {
		return nil
	}

	var lines []string
	for _, ingress := range ingresses {
		lines = append(lines, fmt.Sprintf("%s → %d", ingress.URL, ingress.TargetPort))
	}
	return lines
}

func getStatusIndicator(ready bool) string {
	if ready {
		return "✓"
	}
	return "✗"
}

func getCurrentBuild(builds []v1alpha1.ResourceBuild, res v1alpha1.WorkspaceResource) *v1alpha1.ResourceBuild {
	if res.BuildConfig == nil {
		return nil
	}
	for _, build := range builds {
		if build.WorkspaceResourceName == res.Name && build.SourceHash == res.BuildConfig.ContextDirHash {
			return &build
		}
	}
	return nil
}

func formatRestartStatus(lifecycle *v1alpha1.LifecycleConfig, status *v1alpha1.WorkspaceResourceStatus) string {
	if lifecycle == nil || lifecycle.RestartRequestTime == nil {
		return "-"
	}

	requestTime := lifecycle.RestartRequestTime.UTC()

	if status == nil || status.LastRestartRequestProcessedTime == nil {
		return fmt.Sprintf("Restart pending (%s)", formatDurationShort(time.Since(requestTime)))
	}

	processedTime := status.LastRestartRequestProcessedTime.UTC()

	if processedTime.Before(requestTime) {
		return fmt.Sprintf("Restart pending (%s)", formatDurationShort(time.Since(requestTime)))
	}

	return fmt.Sprintf("Restarted %s ago", formatDurationShort(time.Since(processedTime)))
}

func formatDurationShort(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// Helper functions

// printBuildsForResource prints builds table for a specific resource
func printBuildsForResource(builds []v1alpha1.ResourceBuild, resourceName string) {
	relevantBuilds := filterBuildsByResource(builds, resourceName)
	if len(relevantBuilds) == 0 {
		return
	}

	fmt.Printf("  Builds:\n")
	fmt.Printf("  %-40s %-15s %s\n",
		"ID", "STATE", "SOURCE_HASH")
	fmt.Printf("  %s\n", strings.Repeat("-", 130))

	for _, build := range relevantBuilds {
		state := "Unknown"
		sourceHash := build.SourceHash

		if build.Status != nil {
			if build.Status.State != "" {
				state = build.Status.State
			}
			if build.Status.SourceHash != "" {
				sourceHash = build.Status.SourceHash
			}
		}

		fmt.Printf("  %-40s %-15s %s\n",
			build.ID,
			state,
			sourceHash,
		)
	}
	fmt.Println()
}

func filterBuildsByResource(builds []v1alpha1.ResourceBuild, resourceName string) []v1alpha1.ResourceBuild {
	var filtered []v1alpha1.ResourceBuild
	for _, build := range builds {
		if build.WorkspaceResourceName == resourceName {
			filtered = append(filtered, build)
		}
	}
	return filtered
}

// printIngressDetails prints detailed ingress information
func printIngressDetails(ingresses []v1alpha1.Ingress) {
	fmt.Printf("  Ingress Routes:\n")
	fmt.Printf("  %-50s %s\n", "URL", "PORT")
	fmt.Printf("  %s\n", strings.Repeat("-", 60))
	for _, ing := range ingresses {
		fmt.Printf("  %-50s %d\n",
			ing.URL,
			ing.TargetPort,
		)
	}
	fmt.Println()
}
