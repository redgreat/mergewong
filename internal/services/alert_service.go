// Package services 预警发送服务
// 创建时间: 2025-01
// 创建人: redgreat
//
// 预警策略：
//   - 延迟阈值触发后，每 10 分钟发送一次，连续 3 次后静默 12 小时，再次循环
//   - 执行报错立即发送（不受节流限制）
//   - 恢复后立即发送恢复通知
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

const (
	alertIntervalMinutes = 10
	alertMaxCount        = 3
	alertSilenceHours    = 12
)

type AlertService struct {
	systemDB *gorm.DB
	bot      *WecomBotService
}

// SendTaskAlert 发送任务预警，实现节流：每10分钟一次，3次后静默12小时，循环
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

	// 处于静默期
	if state.SilentAt != nil {
		silenceEnd := state.SilentAt.Add(alertSilenceHours * time.Hour)
		if now.Before(silenceEnd) {
			return nil
		}
		// 静默期结束，重置计数重新开始
		state.AlertCount = 0
		state.SilentAt = nil
	}

	// 检查发送间隔（使用任务配置的冷却时间）
	cooldown := task.AlertCooldownMinutes
	if cooldown < 1 {
		cooldown = alertIntervalMinutes
	}
	if state.LastSentAt != nil && now.Sub(*state.LastSentAt) < time.Duration(cooldown)*time.Minute {
		return nil
	}

	if err := s.bot.SendText(ctx, task.AlertChannel.RobotID, content); err != nil {
		return err
	}

	state.TaskID = task.ID
	state.AlertType = alertType
	state.Active = true
	state.AlertCount++
	state.LastSentAt = &now

	// 达到最大次数，进入静默
	if state.AlertCount >= alertMaxCount {
		state.SilentAt = &now
		state.AlertCount = 0
	}

	if err := s.systemDB.Where("task_id = ? AND alert_type = ?", task.ID, alertType).
		Assign(state).FirstOrCreate(&state).Error; err != nil {
		return err
	}
	NewSyncService().RecordTaskEvent(task, "alert_sent", "alert", "warning", "任务预警已发送",
		fmt.Sprintf("类型：%s；发送群：%s", alertType, task.AlertChannel.Name), 0, 0)
	return nil
}

// SendTaskAlertImmediate 立即发送（跳过节流，用于错误等紧急预警）
func (s *AlertService) SendTaskAlertImmediate(ctx context.Context, task *models.SyncTask, alertType, content string) error {
	if task.AlertChannel == nil || task.AlertChannel.Status != 1 {
		return nil
	}
	if err := s.bot.SendText(ctx, task.AlertChannel.RobotID, content); err != nil {
		return err
	}
	now := time.Now()
	var state models.TaskAlertState
	s.systemDB.Where("task_id = ? AND alert_type = ?", task.ID, alertType).First(&state)
	state.TaskID = task.ID
	state.AlertType = alertType
	state.Active = true
	state.LastSentAt = &now
	s.systemDB.Where("task_id = ? AND alert_type = ?", task.ID, alertType).Assign(state).FirstOrCreate(&state)
	NewSyncService().RecordTaskEvent(task, "alert_sent", "alert", "warning", "任务报错预警已发送",
		fmt.Sprintf("类型：%s；发送群：%s", alertType, task.AlertChannel.Name), 0, 0)
	return nil
}

// ResolveTaskAlert 解除预警状态，并立即发送恢复通知
func (s *AlertService) ResolveTaskAlert(ctx context.Context, task *models.SyncTask, alertType string) error {
	var state models.TaskAlertState
	err := s.systemDB.Where("task_id = ? AND alert_type = ?", task.ID, alertType).First(&state).Error
	if err == gorm.ErrRecordNotFound || !state.Active {
		return nil
	}
	// 立即推送恢复通知
	if task.AlertChannel != nil && task.AlertChannel.Status == 1 {
		content := fmt.Sprintf("数据同步任务恢复通知\n任务：%s\n状态：已恢复正常", task.Name)
		_ = s.bot.SendText(ctx, task.AlertChannel.RobotID, content)
	}
	return s.systemDB.Model(&models.TaskAlertState{}).
		Where("task_id = ? AND alert_type = ?", task.ID, alertType).
		Updates(map[string]interface{}{"active": false, "alert_count": 0, "silent_at": nil}).Error
}

// ResolveTaskAlertSilent 仅重置状态，不发送恢复通知（内部使用）
func (s *AlertService) ResolveTaskAlertSilent(taskID uint, alertType string) error {
	return s.systemDB.Model(&models.TaskAlertState{}).
		Where("task_id = ? AND alert_type = ?", taskID, alertType).
		Updates(map[string]interface{}{"active": false, "alert_count": 0, "silent_at": nil}).Error
}

// CheckTaskAlerts 检测运行延迟和停止预警
func (s *AlertService) CheckTaskAlerts(ctx context.Context) error {
	var tasks []models.SyncTask
	if err := s.systemDB.Preload("AlertChannel").
		Where("status = ? AND alert_channel_id IS NOT NULL", 1).
		Find(&tasks).Error; err != nil {
		return err
	}
	now := time.Now()
	for i := range tasks {
		task := &tasks[i]
		threshold := int64(task.AlertDelaySeconds)

		if task.SyncType == "full" {
			// 全量：以运行时长判断
			if threshold > 0 && task.LastRunStatus == "running" && task.LastRunAt != nil {
				elapsed := int64(now.Sub(*task.LastRunAt).Seconds())
				if elapsed >= threshold {
					content := fmt.Sprintf("数据同步任务延迟预警\n任务：%s\n已运行：%d 秒\n阈值：%d 秒",
						task.Name, elapsed, threshold)
					_ = s.SendTaskAlert(ctx, task, "delay", content)
				} else {
					_ = s.ResolveTaskAlertSilent(task.ID, "delay")
				}
			} else {
				_ = s.ResolveTaskAlertSilent(task.ID, "delay")
			}
		} else {
			// CDC / full_cdc：以 delay_seconds 判断
			if threshold > 0 && task.DelaySeconds >= threshold {
				content := fmt.Sprintf("数据同步任务延迟预警\n任务：%s\n当前延迟：%d 秒\n阈值：%d 秒",
					task.Name, task.DelaySeconds, threshold)
				_ = s.SendTaskAlert(ctx, task, "delay", content)
			} else {
				_ = s.ResolveTaskAlertSilent(task.ID, "delay")
			}
		}

		// 停止预警：仅对定时任务生效，CDC 任务持续运行不适用
		stoppedThreshold := int64(task.AlertStoppedMinutes)
		if stoppedThreshold > 0 && task.SyncType != "cdc" && task.SyncType != "full_cdc" {
			if task.LastRunAt != nil {
				elapsed := int64(now.Sub(*task.LastRunAt).Minutes())
				if task.LastRunStatus != "running" && elapsed >= stoppedThreshold {
					content := fmt.Sprintf("MergeWong 任务停止预警\n任务：%s\n距上次启动：%d 分钟\n阈值：%d 分钟",
						task.Name, elapsed, stoppedThreshold)
					_ = s.SendTaskAlert(ctx, task, "stopped", content)
				} else {
					_ = s.ResolveTaskAlertSilent(task.ID, "stopped")
				}
			} else {
				_ = s.ResolveTaskAlertSilent(task.ID, "stopped")
			}
		} else {
			_ = s.ResolveTaskAlertSilent(task.ID, "stopped")
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
		views = append(views, models.AlertChannelView{
			ID: channel.ID, Name: channel.Name,
			RobotIDMask: maskRobotID(channel.RobotID),
			Status:      channel.Status, CreatedAt: channel.CreatedAt, UpdatedAt: channel.UpdatedAt,
		})
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
	return s.bot.SendText(ctx, channel.RobotID, fmt.Sprintf("数据同步预警发送测试\n发送群：%s", channel.Name))
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
