package main

import (
	"testing"
	"time"

	"github.com/Stackdome/stackdome/pkg/api/openapi"
)

func cond(typ string, t time.Time) openapi.Condition {
	return openapi.Condition{Type: &typ, LastTransitionTime: &t}
}

func buildWith(state string, conds []openapi.Condition) openapi.ImageBuild {
	b := openapi.ImageBuild{Status: &openapi.ImageBuildStatus{Conditions: conds}}
	if state != "" {
		b.Status.State = &state
	}
	return b
}

func TestBuildDurationConditionFallback(t *testing.T) {
	base := time.Date(2026, 8, 5, 20, 38, 0, 0, time.UTC)
	done := base.Add(time.Minute + 41*time.Second)

	created := base.Add(-time.Hour)
	updated := created.Add(30 * time.Second)

	tests := []struct {
		name      string
		build     openapi.ImageBuild
		wantStart *time.Time
		wantDur   string
	}{
		{
			name: "model timestamps win",
			build: openapi.ImageBuild{
				CreatedAt: &created,
				UpdatedAt: &updated,
				Status:    &openapi.ImageBuildStatus{Conditions: []openapi.Condition{cond("BuildJobCreated", base)}},
			},
			wantStart: &created,
			wantDur:   "30s",
		},
		{
			name: "conditions present, terminal",
			build: buildWith("Success", []openapi.Condition{
				cond("BuildJobCreated", base),
				cond("Available", done),
			}),
			wantStart: &base,
			wantDur:   "1m41s",
		},
		{
			name: "failed build uses latest condition as end",
			build: buildWith("Failed", []openapi.Condition{
				cond("BuildJobCreated", base),
				cond("BuildJobFailed", done),
			}),
			wantStart: &base,
			wantDur:   "1m41s",
		},
		{
			name: "only created condition, terminal but no end",
			build: buildWith("Success", []openapi.Condition{
				cond("BuildJobCreated", base),
			}),
			wantStart: &base,
			wantDur:   "-",
		},
		{
			name:      "no conditions at all",
			build:     buildWith("Pending", nil),
			wantStart: nil,
			wantDur:   "-",
		},
		{
			name:      "no status at all",
			build:     openapi.ImageBuild{},
			wantStart: nil,
			wantDur:   "-",
		},
		{
			name: "no named condition falls back to earliest",
			build: buildWith("Success", []openapi.Condition{
				cond("Reconciling", done),
				cond("Initialized", base),
			}),
			wantStart: &base,
			wantDur:   "1m41s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildStartTime(tt.build)
			switch {
			case tt.wantStart == nil && got != nil:
				t.Fatalf("start = %v, want nil", got)
			case tt.wantStart != nil && (got == nil || !got.Equal(*tt.wantStart)):
				t.Fatalf("start = %v, want %v", got, *tt.wantStart)
			}
			if d := buildDuration(tt.build); d != tt.wantDur {
				t.Errorf("duration = %q, want %q", d, tt.wantDur)
			}
			if tt.wantStart == nil && buildStarted(tt.build) != "-" {
				t.Errorf("started = %q, want %q", buildStarted(tt.build), "-")
			}
		})
	}
}

func TestBuildDurationInProgressShowsElapsed(t *testing.T) {
	b := buildWith("Building", []openapi.Condition{
		cond("BuildJobCreated", time.Now().Add(-90*time.Second)),
	})
	if d := buildDuration(b); d != "1m30s" {
		t.Errorf("duration = %q, want %q", d, "1m30s")
	}
}
