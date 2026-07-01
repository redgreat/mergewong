package scheduler

import (
	"testing"

	"github.com/redgreat/mergewong/internal/models"
)

func TestScheduleSpec(t *testing.T) {
	tests := []struct {
		name string
		task models.SyncTask
		want string
		bad  bool
	}{
		{name: "interval", task: models.SyncTask{ScheduleType: "interval", IntervalMinutes: 5}, want: "@every 5m"},
		{name: "cron", task: models.SyncTask{ScheduleType: "cron", CronExpression: "0 * * * *"}, want: "0 * * * *"},
		{name: "manual", task: models.SyncTask{ScheduleType: "manual"}, want: ""},
		{name: "invalid interval", task: models.SyncTask{ScheduleType: "interval"}, bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ScheduleSpec(&tt.task)
			if tt.bad && err == nil {
				t.Fatal("expected error")
			}
			if !tt.bad && (err != nil || got != tt.want) {
				t.Fatalf("got %q, err %v", got, err)
			}
		})
	}
}
