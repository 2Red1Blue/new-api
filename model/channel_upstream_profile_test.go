package model

import "testing"

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
