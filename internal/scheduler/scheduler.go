package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redgreat/mergewong/internal/database"
	"github.com/redgreat/mergewong/internal/models"
	"github.com/redgreat/mergewong/internal/services"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron        *cron.Cron
	tasks       map[uint]cron.EntryID // 任务ID -> Cron EntryID
	mu          sync.RWMutex
	syncService *services.SyncService
}

var (
	instance *Scheduler
	once     sync.Once
)

// GetScheduler 获取调度器单例
func GetScheduler() *Scheduler {
	once.Do(func() {
		instance = &Scheduler{
			cron:        cron.New(),
			tasks:       make(map[uint]cron.EntryID),
			syncService: services.NewSyncService(),
		}
	})
	return instance
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	log.Println("启动定时任务调度器...")

	// 加载所有启用的同步任务
	if err := s.LoadTasks(); err != nil {
		return err
	}

	if _, err := s.cron.AddFunc("@every 1m", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
		defer cancel()
		if err := services.NewAlertService().CheckTaskAlerts(ctx); err != nil {
			log.Printf("任务预警巡检失败: %v", err)
		}
		if err := services.NewServerMonitorService().CheckAlerts(ctx); err != nil {
			log.Printf("服务器预警巡检失败: %v", err)
		}
	}); err != nil {
		return err
	}
	s.cron.Start()
	log.Println("定时任务调度器已启动")
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	log.Println("停止定时任务调度器...")
	s.cron.Stop()
	log.Println("定时任务调度器已停止")
}

// LoadTasks 加载所有启用的任务
func (s *Scheduler) LoadTasks() error {
	db, err := database.GetManager().GetConnection("system")
	if err != nil {
		return err
	}

	var tasks []models.SyncTask
	if err := db.Where("status = ? AND schedule_type != ?", 1, "manual").Find(&tasks).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Println("没有启用的定时同步任务")
			return nil
		}
		return err
	}

	log.Printf("加载了 %d 个定时同步任务", len(tasks))

	for _, task := range tasks {
		if err := s.AddTask(&task); err != nil {
			log.Printf("添加任务失败 [ID: %d]: %v", task.ID, err)
		}
	}

	return nil
}

// AddTask 添加定时任务
func (s *Scheduler) AddTask(task *models.SyncTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	taskID := task.ID

	// 检查任务是否已存在
	if _, exists := s.tasks[taskID]; exists {
		return nil
	}

	// 添加 Cron 任务
	spec, err := ScheduleSpec(task)
	if err != nil {
		return err
	}
	entryID, err := s.cron.AddFunc(spec, func() {
		log.Printf("执行定时同步任务 [ID: %d]", taskID)
		if err := s.syncService.ExecuteTask(taskID); err != nil {
			log.Printf("定时同步任务执行失败 [ID: %d]: %v", taskID, err)
		} else {
			log.Printf("定时同步任务执行成功 [ID: %d]", taskID)
		}
	})

	if err != nil {
		return err
	}

	s.tasks[taskID] = entryID
	log.Printf("添加定时任务 [ID: %d, Schedule: %s]", taskID, spec)
	return nil
}

// RemoveTask 移除定时任务
func (s *Scheduler) RemoveTask(taskID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.tasks[taskID]; exists {
		s.cron.Remove(entryID)
		delete(s.tasks, taskID)
		log.Printf("移除定时任务 [ID: %d]", taskID)
	}
}

// UpdateTask 更新定时任务
func (s *Scheduler) RefreshTask(task *models.SyncTask) error {
	s.RemoveTask(task.ID)
	if task.Status == 0 || task.ScheduleType == "manual" {
		return nil
	}
	return s.AddTask(task)
}

func ScheduleSpec(task *models.SyncTask) (string, error) {
	switch task.ScheduleType {
	case "interval", "":
		if task.IntervalMinutes < 1 {
			return "", fmt.Errorf("执行间隔至少为 1 分钟")
		}
		return fmt.Sprintf("@every %dm", task.IntervalMinutes), nil
	case "cron":
		if task.CronExpression == "" {
			return "", fmt.Errorf("Cron 表达式不能为空")
		}
		return task.CronExpression, nil
	case "manual":
		return "", nil
	default:
		return "", fmt.Errorf("不支持的调度方式: %s", task.ScheduleType)
	}
}
