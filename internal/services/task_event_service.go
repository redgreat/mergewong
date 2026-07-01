package services

import (
	"fmt"
	"time"

	"github.com/redgreat/apiwong/internal/models"
)

func (s *SyncService) RecordTaskEvent(task *models.SyncTask, eventType, phase, status, message, detail string, rows, duration int64) {
	if task == nil {
		return
	}
	_ = s.systemDB.Create(&models.SyncLog{
		TaskID: task.ID, TaskName: task.Name, EventType: eventType, Phase: phase,
		Status: status, Message: message, Detail: detail, ErrorDetail: func() string {
			if status == "failed" {
				return detail
			}
			return ""
		}(),
		RowsAffected: rows, Duration: duration, CreatedAt: time.Now(),
	}).Error
}

func runtimeLabel(status string) string {
	switch status {
	case "initializing":
		return "全量初始化"
	case "catching_up":
		return "增量追数"
	case "cdc_running":
		return "增量同步中"
	case "paused":
		return "暂停"
	case "completed":
		return "完成"
	case "failed":
		return "失败"
	case "pending":
		return "待预检查"
	default:
		return "停止"
	}
}

func (s *SyncService) PauseTask(taskID uint) error {
	task, err := s.GetTask(taskID)
	if err != nil {
		return err
	}
	if task.RuntimeStatus == "paused" {
		return nil
	}
	GetCDCManager().StopTask(taskID)
	if err := s.UpdateTask(taskID, map[string]interface{}{"runtime_status": "paused", "last_run_status": "paused", "last_run_message": "任务已暂停"}); err != nil {
		return err
	}
	s.RecordTaskEvent(task, "task_paused", "control", "success", "任务已暂停", "", 0, 0)
	return nil
}

func (s *SyncService) ResumeTask(taskID uint) error {
	task, err := s.GetTask(taskID)
	if err != nil {
		return err
	}
	if task.RuntimeStatus != "paused" && task.RuntimeStatus != "stopped" && task.RuntimeStatus != "failed" {
		return fmt.Errorf("当前状态不能开始任务")
	}
	if task.ValidationStatus != "passed" {
		return fmt.Errorf("任务预检查尚未通过")
	}
	s.RecordTaskEvent(task, "task_resumed", "control", "success", "任务开始运行", "", 0, 0)
	return s.ExecuteTask(taskID)
}

func (s *SyncService) UpdateBinlogPosition(taskID uint, file string, position uint32) error {
	task, err := s.GetTask(taskID)
	if err != nil {
		return err
	}
	if task.RuntimeStatus != "paused" {
		return fmt.Errorf("只有暂停状态才能修改 Binlog 位点")
	}
	if task.SyncType != "cdc" && task.SyncType != "full_cdc" {
		return fmt.Errorf("该任务不是 Binlog CDC 任务")
	}
	if file == "" || position < 4 {
		return fmt.Errorf("请填写有效的 Binlog file 和 position")
	}
	var checkpoint models.SyncCDCCheckpoint
	if err := s.systemDB.Where("task_id = ?", taskID).First(&checkpoint).Error; err != nil {
		return err
	}
	old := fmt.Sprintf("%s:%d", checkpoint.BinlogFile, checkpoint.BinlogPosition)
	if err := s.systemDB.Model(&checkpoint).Updates(map[string]interface{}{"binlog_file": file, "binlog_position": position, "last_event_at": nil}).Error; err != nil {
		return err
	}
	s.RecordTaskEvent(task, "checkpoint_changed", "control", "success", "Binlog 位点已修改", fmt.Sprintf("%s → %s:%d", old, file, position), 0, 0)
	return nil
}
