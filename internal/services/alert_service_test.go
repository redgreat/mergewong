package services

import (
	"testing"
	"time"

	"github.com/redgreat/mergewong/internal/models"
)

func TestTaskCurrentDelaySeconds(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	lastSuccess := now.Add(-2 * time.Hour)
	lastRun := now.Add(-30 * time.Minute)
	started := now.Add(-5 * time.Minute)

	tests := []struct {
		name string
		task models.SyncTask
		want int64
	}{
		{name: "cdc delay", task: models.SyncTask{SyncType: "full_cdc", RuntimeStatus: "cdc_running", DelaySeconds: 42}, want: 42},
		{name: "paused from success", task: models.SyncTask{SyncType: "full_cdc", RuntimeStatus: "paused", LastSuccessAt: &lastSuccess, DelaySeconds: 42}, want: 7200},
		{name: "stopped from last run", task: models.SyncTask{SyncType: "full_cdc", RuntimeStatus: "stopped", LastRunAt: &lastRun}, want: 1800},
		{name: "initializing elapsed", task: models.SyncTask{SyncType: "full_cdc", RuntimeStatus: "initializing", PhaseStartedAt: &started}, want: 300},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := taskCurrentDelaySeconds(&tt.task, now); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
