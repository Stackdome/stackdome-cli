package main

import (
	"testing"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
)

func healthyDeployObservation(public bool) (*openapi.StackReleaseDetail, *openapi.Stack, *openapi.ReleaseLiveStatus) {
	releaseID := "release-requested"
	state := openapi.RELEASE_STATE_RELEASED
	health := openapi.RELEASE_HEALTH_OK
	ports := []openapi.Port{{Name: "http", Number: 80, ExposedToPublic: public}}
	stack := &openapi.Stack{
		Name: "app",
		Spec: openapi.StackSpec{StackResources: []openapi.StackResource{{
			Name:  "web",
			Ports: ports,
		}}},
		ConvergedRelease: &openapi.ReleaseSummary{Id: &releaseID, State: &state},
	}
	statuses := map[string]openapi.StackResourceStatus{}
	if public {
		statuses["web"] = openapi.StackResourceStatus{PublicIngress: []openapi.Ingress{{
			Url:        openapi.PtrString("https://app.example"),
			TargetPort: openapi.PtrInt32(80),
		}}}
	}
	return &openapi.StackReleaseDetail{Id: &releaseID, State: &state}, stack, &openapi.ReleaseLiveStatus{
		Health:    &health,
		Resources: &statuses,
	}
}

func TestVerifyDeployObservationEnforcesAgentSuccessContract(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*openapi.StackReleaseDetail, *openapi.Stack, *openapi.ReleaseLiveStatus)
		public bool
		wantOK bool
	}{
		{name: "healthy private stack", public: false, wantOK: true},
		{name: "healthy public stack", public: true, wantOK: true},
		{name: "release not terminal Released", public: false, mutate: func(r *openapi.StackReleaseDetail, _ *openapi.Stack, _ *openapi.ReleaseLiveStatus) {
			state := openapi.RELEASE_STATE_FAILED
			r.State = &state
		}},
		{name: "fetched release ID mismatch", public: false, mutate: func(r *openapi.StackReleaseDetail, _ *openapi.Stack, _ *openapi.ReleaseLiveStatus) {
			r.Id = openapi.PtrString("release-other")
		}},
		{name: "missing converged release", public: false, mutate: func(_ *openapi.StackReleaseDetail, s *openapi.Stack, _ *openapi.ReleaseLiveStatus) {
			s.ConvergedRelease = nil
		}},
		{name: "converged release mismatch", public: false, mutate: func(_ *openapi.StackReleaseDetail, s *openapi.Stack, _ *openapi.ReleaseLiveStatus) {
			s.ConvergedRelease.Id = openapi.PtrString("release-other")
		}},
		{name: "missing live status", public: false, mutate: func(_ *openapi.StackReleaseDetail, _ *openapi.Stack, live *openapi.ReleaseLiveStatus) {
			*live = openapi.ReleaseLiveStatus{}
		}},
		{name: "degraded runtime", public: false, mutate: func(_ *openapi.StackReleaseDetail, _ *openapi.Stack, live *openapi.ReleaseLiveStatus) {
			health := openapi.RELEASE_HEALTH_DEGRADED
			live.Health = &health
		}},
		{name: "public URL missing", public: true, mutate: func(_ *openapi.StackReleaseDetail, _ *openapi.Stack, live *openapi.ReleaseLiveStatus) {
			statuses := map[string]openapi.StackResourceStatus{"web": {}}
			live.Resources = &statuses
		}},
		{name: "wrong public target port", public: true, mutate: func(_ *openapi.StackReleaseDetail, _ *openapi.Stack, live *openapi.ReleaseLiveStatus) {
			statuses := map[string]openapi.StackResourceStatus{"web": {PublicIngress: []openapi.Ingress{{
				Url: openapi.PtrString("https://app.example"), TargetPort: openapi.PtrInt32(8080),
			}}}}
			live.Resources = &statuses
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release, stack, live := healthyDeployObservation(tt.public)
			if tt.mutate != nil {
				tt.mutate(release, stack, live)
			}
			err := verifyDeployObservation("release-requested", release, stack, live)
			if tt.wantOK && err != nil {
				t.Fatalf("verifyDeployObservation: %v", err)
			}
			if !tt.wantOK && err == nil {
				t.Fatal("verifyDeployObservation returned nil, want verification failure")
			}
		})
	}
}
