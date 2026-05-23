package model

import (
	"strings"
	"testing"
)

func TestPrimaryChannelKeyReturnsFirstKey(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "multi key", raw: "sk-first\nsk-second\nsk-third", want: "sk-first"},
		{name: "single key", raw: "  sk-only  ", want: "sk-only"},
		{name: "empty", raw: "   ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := primaryChannelKey(tt.raw); got != tt.want {
				t.Fatalf("primaryChannelKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCalculateAutoPriorityValueUsesEffectiveRatio(t *testing.T) {
	tests := []struct {
		name       string
		groupRatio float64
		topupRatio float64
		want       int64
	}{
		{name: "cheap channel gets high priority", groupRatio: 0.1, topupRatio: 1, want: 10},
		{name: "very cheap channel hits cap", groupRatio: 0.01, topupRatio: 1, want: 100},
		{name: "normal channel uses base", groupRatio: 1, topupRatio: 1, want: 1},
		{name: "expensive channel can reach zero after rounding", groupRatio: 2, topupRatio: 1, want: 1},
		{name: "topup ratio lowers effective cost", groupRatio: 1.7, topupRatio: 16.6667, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := CalculateAutoPriorityValue(tt.groupRatio, tt.topupRatio, 1, 1, 100)
			if got != tt.want {
				t.Fatalf("CalculateAutoPriorityValue() = %d, want %d, reason=%s", got, tt.want, reason)
			}
			if !strings.Contains(reason, "priority=") {
				t.Fatalf("reason should include priority, got %q", reason)
			}
		})
	}
}

func TestCalculateAutoPriorityValueClampsAndSkipsInvalidRatio(t *testing.T) {
	got, _ := CalculateAutoPriorityValue(0.001, 1, 1, 0, 50)
	if got != 50 {
		t.Fatalf("expected max clamp 50, got %d", got)
	}

	got, reason := CalculateAutoPriorityValue(0, 1, 10, 0, 100)
	if got != 0 {
		t.Fatalf("expected zero priority for invalid ratio, got %d", got)
	}
	if !strings.Contains(reason, "skip") {
		t.Fatalf("expected skip reason, got %q", reason)
	}
}
