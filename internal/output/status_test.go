package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
)

func TestRenderStackStatusSeparatesLatestRolloutFromServingRelease(t *testing.T) {
	SetNoColor(true)
	t.Cleanup(func() { SetNoColor(false) })

	now := time.Now()
	latestID := "ad1642ba-1111-2222-3333-444444444444"
	servingID := "56cd2d3e-1111-2222-3333-444444444444"
	latestSequence, servingSequence := int32(2), int32(1)
	latestState := openapi.RELEASE_STATE_IN_PROGRESS
	servingState := openapi.RELEASE_STATE_RELEASED
	health := openapi.RELEASE_HEALTH_PROGRESSING
	message := "waiting for the new workload"
	resourceState := "Ready"
	resources := map[string]openapi.StackResourceStatus{
		"n8n": {State: &resourceState},
	}
	stack := &openapi.Stack{
		Name: "n8n",
		Spec: openapi.StackSpec{StackResources: []openapi.StackResource{{Name: "n8n"}}},
		LatestRelease: &openapi.ReleaseSummary{
			Id:        &latestID,
			Sequence:  &latestSequence,
			State:     &latestState,
			Message:   &message,
			CreatedAt: ptrTime(now.Add(-9*time.Minute - 10*time.Second)),
		},
		ConvergedRelease: &openapi.ReleaseSummary{
			Id:       &servingID,
			Sequence: &servingSequence,
			State:    &servingState,
		},
	}

	var got bytes.Buffer
	RenderStackStatus(&got, stack, &openapi.ReleaseLiveStatus{Health: &health, Resources: &resources}, false)
	out := got.String()
	for _, want := range []string{
		"Stack: n8n",
		"Latest:", "#2", "InProgress", "ad1642ba", "9m ago",
		"Latest message:", message,
		"Serving:", "#1", "Released", "56cd2d3e",
		"Serving health:", "progressing",
		"Serving resources:", "n8n", "Ready",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "State: Released") {
		t.Errorf("status output still has misleading converged-release header:\n%s", out)
	}
}

func TestRenderStackStatusCollapsesConvergedLatestRelease(t *testing.T) {
	SetNoColor(true)
	t.Cleanup(func() { SetNoColor(false) })

	releaseID := "d8c636b5-1111-2222-3333-444444444444"
	sequence := int32(3)
	state := openapi.RELEASE_STATE_RELEASED
	health := openapi.RELEASE_HEALTH_OK
	release := &openapi.ReleaseSummary{Id: &releaseID, Sequence: &sequence, State: &state}
	stack := &openapi.Stack{
		Name:             "n8n",
		Spec:             openapi.StackSpec{},
		LatestRelease:    release,
		ConvergedRelease: &openapi.ReleaseSummary{Id: &releaseID, Sequence: &sequence, State: &state},
	}

	var got bytes.Buffer
	RenderStackStatus(&got, stack, &openapi.ReleaseLiveStatus{Health: &health}, false)
	out := got.String()
	for _, want := range []string{"Stack: n8n", "Release:", "#3", "Released", "d8c636b5", "Health:", "ok"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Latest:") || strings.Contains(out, "Serving:") {
		t.Errorf("converged latest release was not collapsed:\n%s", out)
	}
	for _, verboseLabel := range []string{"Serving health:", "Serving resources:", "Latest message:"} {
		if strings.Contains(out, verboseLabel) {
			t.Errorf("converged latest release contains split-release label %q:\n%s", verboseLabel, out)
		}
	}
}

func TestStackReleasePrefersLatestRelease(t *testing.T) {
	latestID := "latest"
	servingID := "serving"
	stack := &openapi.Stack{
		LatestRelease:    &openapi.ReleaseSummary{Id: &latestID},
		ConvergedRelease: &openapi.ReleaseSummary{Id: &servingID},
	}

	got := StackRelease(stack)
	if got == nil || got.Id == nil || *got.Id != latestID {
		t.Fatalf("StackRelease() = %#v, want latest release", got)
	}
}

func ptrTime(v time.Time) *time.Time { return &v }

func TestFormatPortsDefaultsProtocol(t *testing.T) {
	empty := ""
	tcp := "TCP"
	got := formatPorts([]openapi.Port{
		{Number: 80, Protocol: &empty},
		{Number: 443},
		{Number: 5432, Protocol: &tcp},
	})
	want := "80/HTTP, 443/HTTP, 5432/TCP"
	if got != want {
		t.Errorf("formatPorts = %q, want %q", got, want)
	}
}
