package messaging

import (
	"testing"
	"time"
)

func TestParseRetentionPeriod(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		enabled  bool
		duration time.Duration
	}{
		{"empty disables", "", false, 0},
		{"zero disables", "0", false, 0},
		{"12 months", "12m", true, 12 * 30 * 24 * time.Hour},
		{"6 months", "6m", true, 6 * 30 * 24 * time.Hour},
		{"1 month", "1m", true, 30 * 24 * time.Hour},
		{"365 days", "365d", true, 365 * 24 * time.Hour},
		{"30 days", "30d", true, 30 * 24 * time.Hour},
		{"go duration hours", "8760h", true, 8760 * time.Hour},
		{"invalid disables", "invalid", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ParseRetentionPeriod(tt.input)
			if cfg.Enabled != tt.enabled {
				t.Errorf("Enabled = %v, want %v", cfg.Enabled, tt.enabled)
			}
			if cfg.RetentionPeriod != tt.duration {
				t.Errorf("RetentionPeriod = %v, want %v", cfg.RetentionPeriod, tt.duration)
			}
		})
	}
}

func TestRetentionPeriodHuman(t *testing.T) {
	tests := []struct {
		name   string
		config RetentionConfig
		want   string
	}{
		{"disabled", RetentionConfig{Enabled: false}, "disabled"},
		{"1 month", RetentionConfig{Enabled: true, RetentionPeriod: 30 * 24 * time.Hour}, "1 month"},
		{"12 months", RetentionConfig{Enabled: true, RetentionPeriod: 360 * 24 * time.Hour}, "12 months"},
		{"45 days", RetentionConfig{Enabled: true, RetentionPeriod: 45 * 24 * time.Hour}, "45 days"},
		{"1 day", RetentionConfig{Enabled: true, RetentionPeriod: 24 * time.Hour}, "1 day"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.RetentionPeriodHuman()
			if got != tt.want {
				t.Errorf("RetentionPeriodHuman() = %q, want %q", got, tt.want)
			}
		})
	}
}
