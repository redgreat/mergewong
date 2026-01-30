package scheduler

import (
	"log"
	"sync"

	"github.com/redgreat/apiwong/internal/database"
	"github.com/redgreat/apiwong/internal/models"
	"github.com/redgreat/apiwong/internal/services"
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
	if err := db.Where("status = ? AND cron_expression != ?", 1, "").Find(&tasks).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Println("没有启用的定时同步任务")
			return nil
		}
		return err
	}

	log.Printf("加载了 %d 个定时同步任务", len(tasks))

	for _, task := range tasks {
		if err := s.AddTask(task.ID, task.CronExpression); err != nil {
			log.Printf("添加任务失败 [ID: %d]: %v", task.ID, err)
		}
	}

	return nil
}

// AddTask 添加定时任务
func (s *Scheduler) AddTask(taskID uint, cronExpression string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查任务是否已存在
	if _, exists := s.tasks[taskID]; exists {
		return nil
	}

	// 添加 Cron 任务
	entryID, err := s.cron.AddFunc(cronExpression, func() {
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
	log.Printf("添加定时任务 [ID: %d, Cron: %s]", taskID, cronExpression)
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
func (s *Scheduler) UpdateTask(taskID uint, cronExpression string) error {
	s.RemoveTask(taskID)
	return s.AddTask(taskID, cronExpression)
}
