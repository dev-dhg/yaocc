package test

import (
	"testing"
	"time"

	"github.com/dev-dhg/yaocc/pkg/agent"
	"github.com/dev-dhg/yaocc/pkg/config"
	"github.com/dev-dhg/yaocc/pkg/cron"
	"github.com/dev-dhg/yaocc/pkg/messaging"
)

func TestNewScheduler_Timezone(t *testing.T) {
	// Mock dependencies
	a := &agent.Agent{}
	providers := make(map[string]messaging.Provider)

	t.Run("Default Timezone (Empty)", func(t *testing.T) {
		cfg := &config.Config{}
		s := cron.NewScheduler(cfg, ".", a, providers)
		if s.Cron.Location() != time.Local {
			t.Errorf("Expected Local timezone, got %v", s.Cron.Location())
		}
	})

	t.Run("Valid Timezone (UTC)", func(t *testing.T) {
		cfg := &config.Config{
			Timezone: "UTC",
		}
		s := cron.NewScheduler(cfg, ".", a, providers)
		if s.Cron.Location().String() != "UTC" {
			t.Errorf("Expected UTC timezone, got %v", s.Cron.Location())
		}
	})

	t.Run("Valid Timezone (America/New_York)", func(t *testing.T) {
		locName := "America/New_York"
		// Ensure system has this timezone or skip
		loc, err := time.LoadLocation(locName)
		if err != nil {
			t.Skipf("Skipping test: timezone %s not found on system", locName)
		}

		cfg := &config.Config{
			Timezone: locName,
		}
		s := cron.NewScheduler(cfg, ".", a, providers)
		if s.Cron.Location().String() != loc.String() {
			t.Errorf("Expected %s timezone, got %v", locName, s.Cron.Location())
		}
	})

	t.Run("Invalid Timezone Fallback", func(t *testing.T) {
		cfg := &config.Config{
			Timezone: "Invalid/Timezone",
		}
		s := cron.NewScheduler(cfg, ".", a, providers)
		// Should fall back to Local
		if s.Cron.Location() != time.Local {
			t.Errorf("Expected Local timezone fallback, got %v", s.Cron.Location())
		}
	})
}
