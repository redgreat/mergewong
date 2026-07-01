package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redgreat/mergewong/internal/database"
	"github.com/redgreat/mergewong/internal/models"
	"gorm.io/gorm"
)

type AlertService struct {
	systemDB *gorm.DB
	bot      *WecomBotService
}

// SendTaskAlert sends on first activation and then at the configured cooldown interval.
func (s *AlertService) SendTaskAlert(ctx context.Context, task *models.SyncTask, alertType, content string) error {
	if task.AlertChannel == nil || task.AlertChannel.Status != 1 {
		return nil
	}
	var state models.TaskAlertState
	err := s.systemDB.Where("task_id = ? AND alert_type = ?", task.ID, alertType).First(&state).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	now := time.Now()
	cooldown := time.Duration(task.AlertCooldownMinutes) * time.Minute
	if cooldown <= 0 {
		cooldown = 30 * time.Minute
	}
	if state.Active && state.LastSentAt != nil && now.Sub(*state.LastSentAt) < cooldown {
		return nil
	}
	if err := s.bot.SendText(ctx, task.AlertChannel.RobotID, content); err != nil {
		return err
	}
	state.TaskID, state.AlertType, state.Active, state.LastSentAt = task.ID, alertType, true, &now
	if err := s.systemDB.Where("task_id = ? AND alert_type = ?", task.ID, alertType).Assign(state).FirstOrCreate(&state).Error; err != nil {
		return err
	}
	NewSyncService().RecordTaskEvent(task, "alert_sent", "alert", "warning", "任务预警已发送", fmt.Sprintf("类型：%s；发送群：%s", alertType, task.AlertChannel.Name), 0, 0)
	return nil
}

func (s *AlertService) ResolveTaskAlert(taskID uint, alertType string) error {
	return s.systemDB.Model(&models.TaskAlertState{}).Where("task_id = ? AND alert_type = ?", taskID, alertType).Update("active", false).Error
}

// CheckTaskAlerts detects long-running and not-started scheduled tasks.
func (s *AlertService) CheckTaskAlerts(ctx context.Context) error {
	var tasks []models.SyncTask
	if err := s.systemDB.Preload("AlertChannel").Where("status = ? AND alert_channel_id IS NOT NULL", 1).Find(&tasks).Error; err != nil {
		return err
	}
	now := time.Now()
	for i := range tasks {
		task := &tasks[i]
		if task.SyncType == "full" && task.AlertDelayMinutes > 0 && task.LastRunStatus == "running" && task.LastRunAt != nil {
			elapsed := now.Sub(*task.LastRunAt)
			if elapsed >= time.Duration(task.AlertDelayMinutes)*time.Minute {
				content := fmt.Sprintf("MergeWong 任务延迟预警\n任务：%s\n已运行：%d 分钟\n阈值：%d 分钟", task.Name, int(elapsed.Minutes()), task.AlertDelayMinutes)
				if err := s.SendTaskAlert(ctx, task, "delay", content); err != nil {
					return err
				}
			} else {
				_ = s.ResolveTaskAlert(task.ID, "delay")
			}
		} else {
			cdcTask := task.SyncType == "cdc" || task.SyncType == "full_cdc"
			if cdcTask && task.AlertDelayMinutes > 0 && task.DelaySeconds >= int64(task.AlertDelayMinutes*60) {
				content := fmt.Sprintf("MergeWong 任务延迟预警\n任务：%s\n当前延迟：%d 秒\n阈值：%d 分钟", task.Name, task.DelaySeconds, task.AlertDelayMinutes)
				if err := s.SendTaskAlert(ctx, task, "delay", content); err != nil {
					return err
				}
			} else {
				_ = s.ResolveTaskAlert(task.ID, "delay")
			}
		}

		cdcTask := task.SyncType == "cdc" || task.SyncType == "full_cdc"
		if task.AlertStoppedMinutes > 0 && cdcTask && !GetCDCManager().IsRunning(task.ID) {
			baseline := task.CreatedAt
			if task.LastRunAt != nil {
				baseline = *task.LastRunAt
			}
			elapsed := now.Sub(baseline)
			if elapsed >= time.Duration(task.AlertStoppedMinutes)*time.Minute {
				content := fmt.Sprintf("MergeWong 任务停止预警\n任务：%s\n距上次启动：%d 分钟\n阈值：%d 分钟", task.Name, int(elapsed.Minutes()), task.AlertStoppedMinutes)
				if err := s.SendTaskAlert(ctx, task, "stopped", content); err != nil {
					return err
				}
			} else {
				_ = s.ResolveTaskAlert(task.ID, "stopped")
			}
		} else {
			_ = s.ResolveTaskAlert(task.ID, "stopped")
		}
	}
	return nil
}

func NewAlertService() *AlertService {
	db, _ := database.GetManager().GetConnection("system")
	return &AlertService{systemDB: db, bot: NewWecomBotService()}
}

func (s *AlertService) Create(channel *models.AlertChannel) error {
	return s.systemDB.Create(channel).Error
}

func (s *AlertService) Get(id uint) (*models.AlertChannel, error) {
	var channel models.AlertChannel
	if err := s.systemDB.First(&channel, id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (s *AlertService) List(page, pageSize int, enabledOnly bool) ([]models.AlertChannelView, int64, error) {
	query := s.systemDB.Model(&models.AlertChannel{})
	if enabledOnly {
		query = query.Where("status = ?", 1)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var channels []models.AlertChannel
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&channels).Error; err != nil {
		return nil, 0, err
	}
	views := make([]models.AlertChannelView, 0, len(channels))
	for _, channel := range channels {
		views = append(views, models.AlertChannelView{ID: channel.ID, Name: channel.Name, RobotIDMask: maskRobotID(channel.RobotID), Status: channel.Status, CreatedAt: channel.CreatedAt, UpdatedAt: channel.UpdatedAt})
	}
	return views, total, nil
}

func (s *AlertService) Update(id uint, updates map[string]interface{}) error {
	return s.systemDB.Model(&models.AlertChannel{}).Where("id = ?", id).Updates(updates).Error
}

func (s *AlertService) Delete(id uint) error {
	var count int64
	if err := s.systemDB.Model(&models.SyncTask{}).Where("alert_channel_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该发送方仍被 %d 个同步任务使用", count)
	}
	return s.systemDB.Delete(&models.AlertChannel{}, id).Error
}

func (s *AlertService) Test(ctx context.Context, id uint) error {
	channel, err := s.Get(id)
	if err != nil {
		return err
	}
	return s.bot.SendText(ctx, channel.RobotID, fmt.Sprintf("MergeWong 预警发送测试\n发送群：%s", channel.Name))
}

func maskRobotID(value string) string {
	value = strings.TrimSpace(value)
	if key, err := getWecomKey(value); err == nil {
		value = key
	}
	if len(value) <= 8 {
		return "********"
	}
	return value[:4] + "****" + value[len(value)-4:]
}
