package services

import (
	"testing"

	"github.com/redgreat/mergewong/internal/models"
)

func TestIsFullSnapshotTask(t *testing.T) {
	tables := []models.SyncTaskTable{{ID: 1}}
	tests := []struct {
		name string
		task models.SyncTask
		want bool
	}{
		{name: "全量任务带表映射", task: models.SyncTask{SyncType: "full", TaskTables: tables}, want: true},
		{name: "全量任务无表映射", task: models.SyncTask{SyncType: "full"}, want: false},
		{name: "CDC 任务", task: models.SyncTask{SyncType: "full_cdc", TaskTables: tables}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFullSnapshotTask(&tt.task); got != tt.want {
				t.Fatalf("isFullSnapshotTask = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyScheduledRun(t *testing.T) {
	tests := []struct {
		name string
		task *models.SyncTask
		want scheduledRunAction
	}{
		{name: "上一轮已完成仍然执行新一轮", task: &models.SyncTask{SyncType: "full", Status: 1, RuntimeStatus: "completed"}, want: scheduledRunExecute},
		{name: "上一轮失败下次调度重试", task: &models.SyncTask{SyncType: "full", Status: 1, RuntimeStatus: "failed"}, want: scheduledRunExecute},
		{name: "进程重启后残留 initializing 仍执行", task: &models.SyncTask{SyncType: "full", Status: 1, RuntimeStatus: "initializing"}, want: scheduledRunExecute},
		{name: "暂停任务跳过等待手动开始", task: &models.SyncTask{SyncType: "full", Status: 1, RuntimeStatus: "paused"}, want: scheduledRunSkipPaused},
		{name: "禁用任务跳过", task: &models.SyncTask{SyncType: "full", Status: 0, RuntimeStatus: "completed"}, want: scheduledRunSkipIrrelevant},
		{name: "CDC 任务不参与定时调度", task: &models.SyncTask{SyncType: "full_cdc", Status: 1}, want: scheduledRunSkipIrrelevant},
		{name: "空任务跳过", task: nil, want: scheduledRunSkipIrrelevant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyScheduledRun(tt.task); got != tt.want {
				t.Fatalf("classifyScheduledRun = %v, want %v", got, tt.want)
			}
		})
	}
}
