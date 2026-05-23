package service

import (
	"testing"
	"time"
)

func TestNormalizeAutoPriorityScanInterval(t *testing.T) {
	tests := []struct {
		name  string
		hours float64
		want  time.Duration
	}{
		{name: "default on zero", hours: 0, want: 6 * time.Hour},
		{name: "default on negative", hours: -1, want: 6 * time.Hour},
		{name: "half hour", hours: 0.5, want: 30 * time.Minute},
		{name: "rounds to minutes", hours: 1.25, want: 75 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAutoPriorityScanInterval(tt.hours); got != tt.want {
				t.Fatalf("normalizeAutoPriorityScanInterval() = %s, want %s", got, tt.want)
			}
		})
	}
}
