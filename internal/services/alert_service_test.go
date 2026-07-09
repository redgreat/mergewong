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
		{name: "cdc running delay", task: models.SyncTask{SyncType: "full_cdc", RuntimeStatus: "cdc_running", DelaySeconds: 42}, want: 42},
		{name: "catching up delay", task: models.SyncTask{SyncType: "full_cdc", RuntimeStatus: "catching_up", DelaySeconds: 35}, want: 35},
		{name: "paused ignored", task: models.SyncTask{SyncType: "full_cdc", RuntimeStatus: "paused", LastSuccessAt: &lastSuccess, DelaySeconds: 42}, want: 0},
		{name: "stopped ignored", task: models.SyncTask{SyncType: "full_cdc", RuntimeStatus: "stopped", LastRunAt: &lastRun, DelaySeconds: 42}, want: 0},
		{name: "initializing ignored", task: models.SyncTask{SyncType: "full_cdc", RuntimeStatus: "initializing", PhaseStartedAt: &started, DelaySeconds: 42}, want: 0},
		{name: "failed ignored", task: models.SyncTask{SyncType: "full_cdc", RuntimeStatus: "failed", LastRunAt: &lastRun, LastRunMessage: "binlog disconnected", DelaySeconds: 42}, want: 0},
		{name: "full sync ignored", task: models.SyncTask{SyncType: "full", RuntimeStatus: "cdc_running", DelaySeconds: 42}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := taskCurrentDelaySeconds(&tt.task, now); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
