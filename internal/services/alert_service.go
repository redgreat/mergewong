// Package services 预警发送服务
// 创建时间: 2025-01
// 创建人: redgreat
//
// 预警策略：
//   - 只按同步延迟阈值触发，不区分暂停、停止或追数延迟原因
//   - 延迟超限后立即提醒一次，之后分别间隔 1 小时、3 小时、6 小时再提醒
//   - 第 4 次提醒后不再重复提醒，直到延迟恢复到阈值内并重置状态
//   - 执行报错立即发送（不受节流限制）
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
	finalDelayAlertWarning = "本次为该轮延迟超限的最后一次预警，后续不再重复提醒；请尽快处理或确认任务已恢复。"
)

type AlertService struct {
	systemDB *gorm.DB
	bot      *WecomBotService
}

// SendTaskAlert 发送延迟预警：首次立即发送，后续按 1h、3h、6h 提醒，最终提醒后静默。
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

	if state.SilentAt != nil {
		return nil
	}

	if state.LastSentAt != nil && state.AlertCount < len(delayAlertIntervals) && now.Sub(*state.LastSentAt) < delayAlertIntervals[state.AlertCount] {
		return nil
	}
	if state.AlertCount >= len(delayAlertIntervals) {
		return nil
	}
	if state.AlertCount == len(delayAlertIntervals)-1 {
		content += "\n" + finalDelayAlertWarning
	}

	if err := s.bot.SendText(ctx, task.AlertChannel.RobotID, content); err != nil {
		return err
	}

	state.TaskID = task.ID
	state.AlertType = alertType
	state.Active = true
	state.AlertCount++
	state.LastSentAt = &now

	if state.AlertCount >= len(delayAlertIntervals) {
		state.SilentAt = &now
	}

	if err := s.systemDB.Where("task_id = ? AND alert_type = ?", task.ID, alertType).
		Assign(state).FirstOrCreate(&state).Error; err != nil {
		return err
	}
	NewSyncService().RecordTaskEvent(task, "alert_sent", "alert", "warning", "任务预警已发送",
		fmt.Sprintf("类型：%s；发送群：%s", alertType, task.AlertChannel.Name), 0, 0)
	return nil
}

var delayAlertIntervals = []time.Duration{
	0,
	time.Hour,
	3 * time.Hour,
	6 * time.Hour,
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

// CheckTaskAlerts 检测同步延迟预警。
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
		delay := taskCurrentDelaySeconds(task, now)
		if threshold > 0 && delay >= threshold {
			content := fmt.Sprintf("数据同步任务延迟预警\n任务：%s\n当前延迟：%d 秒\n阈值：%d 秒\n状态：%s",
				task.Name, delay, threshold, runtimeLabel(task.RuntimeStatus))
			_ = s.SendTaskAlert(ctx, task, "delay", content)
		} else {
			_ = s.ResolveTaskAlertSilent(task.ID, "delay")
		}
		_ = s.ResolveTaskAlertSilent(task.ID, "stopped")
	}
	return nil
}

func taskCurrentDelaySeconds(task *models.SyncTask, now time.Time) int64 {
	if task.SyncType == "full" && task.LastRunStatus == "running" && task.LastRunAt != nil {
		return int64(now.Sub(*task.LastRunAt).Seconds())
	}
	if task.RuntimeStatus == "initializing" && task.PhaseStartedAt != nil {
		return int64(now.Sub(*task.PhaseStartedAt).Seconds())
	}
	if task.RuntimeStatus == "paused" || task.RuntimeStatus == "stopped" {
		if task.LastSuccessAt != nil {
			return int64(now.Sub(*task.LastSuccessAt).Seconds())
		}
		if task.LastRunAt != nil {
			return int64(now.Sub(*task.LastRunAt).Seconds())
		}
	}
	return task.DelaySeconds
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
