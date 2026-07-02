package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestParseUpstreamGroupRatiosSupportsFlatAndSnapshotFormats(t *testing.T) {
	t.Run("flat map", func(t *testing.T) {
		ratios := ParseUpstreamGroupRatios(`{"default":1,"特惠分组":0.06}`)
		require.NotNil(t, ratios)
		assert.Equal(t, map[string]float64{
			"default": 1,
			"特惠分组":    0.06,
		}, ratios)
	})

	t.Run("snapshot map", func(t *testing.T) {
		ratios := ParseUpstreamGroupRatios(`{
			"稳定puls":{"rate_multiplier":0.95,"platform":"openai"},
			"Kiro高缓":{"rate_multiplier":2,"platform":"anthropic"}
		}`)
		require.NotNil(t, ratios)
		assert.Equal(t, map[string]float64{
			"稳定puls": 0.95,
			"Kiro高缓": 2,
		}, ratios)
	})
}

func TestIsUpstreamGroupMissingSupportsSnapshotFormat(t *testing.T) {
	raw := `{
		"稳定puls":{"rate_multiplier":0.95,"platform":"openai"},
		"Kiro高缓":{"rate_multiplier":2,"platform":"anthropic"}
	}`

	assert.False(t, IsUpstreamGroupMissing(raw, "稳定puls"))
	assert.True(t, HasUpstreamGroupRatio(raw, "Kiro高缓"))
	assert.True(t, IsUpstreamGroupMissing(raw, "不存在的分组"))
}
