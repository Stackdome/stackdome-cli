package main

import (
	"testing"
	"time"
)

func TestParseExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		in      string
		want    *time.Time
		wantErr bool
	}{
		{in: "", want: nil},
		{in: "720h", want: ptr(now.Add(720 * time.Hour))},
		{in: "30m", want: ptr(now.Add(30 * time.Minute))},
		{in: "30d", wantErr: true}, // Go durations have no day unit
		{in: "yesterday", wantErr: true},
		{in: "0h", wantErr: true},
		{in: "-1h", wantErr: true},
	}

	for _, tc := range tests {
		got, err := parseExpiry(tc.in, now)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseExpiry(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseExpiry(%q): %v", tc.in, err)
			continue
		}
		if (got == nil) != (tc.want == nil) || (got != nil && !got.Equal(*tc.want)) {
			t.Errorf("parseExpiry(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func ptr(t time.Time) *time.Time { return &t }
