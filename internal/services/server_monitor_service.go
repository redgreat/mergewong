package services

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/redgreat/mergewong/internal/database"
	"github.com/redgreat/mergewong/internal/models"
	"gorm.io/gorm"
)

type ServerMonitorService struct {
	systemDB *gorm.DB
	bot      *WecomBotService
}

type ServerMetrics struct {
	CPUPercent      float64             `json:"cpu_percent"`
	MemoryPercent   float64             `json:"memory_percent"`
	MemoryTotal     uint64              `json:"memory_total"`
	MemoryUsed      uint64              `json:"memory_used"`
	MemoryAvailable uint64              `json:"memory_available"`
	DiskPercent     float64             `json:"disk_percent"`
	DiskTotal       uint64              `json:"disk_total"`
	DiskUsed        uint64              `json:"disk_used"`
	ProcessMemory   uint64              `json:"process_memory"`
	Goroutines      int                 `json:"goroutines"`
	NumCPU          int                 `json:"num_cpu"`
	DBPools         []DatabasePoolStats `json:"db_pools"`
	CollectedAt     time.Time           `json:"collected_at"`
}

type DatabasePoolStats struct {
	Name           string `json:"name"`
	Open           int    `json:"open"`
	InUse          int    `json:"in_use"`
	Idle           int    `json:"idle"`
	MaxOpen        int    `json:"max_open"`
	WaitCount      int64  `json:"wait_count"`
	WaitDurationMS int64  `json:"wait_duration_ms"`
}

func NewServerMonitorService() *ServerMonitorService {
	db, _ := database.GetManager().GetConnection("system")
	return &ServerMonitorService{systemDB: db, bot: NewWecomBotService()}
}

func (s *ServerMonitorService) Metrics() (*ServerMetrics, error) {
	cpuPercent, _ := systemCPUPercent()
	mem, err := systemMemory()
	if err != nil {
		return nil, err
	}
	disk, err := diskUsage(".")
	if err != nil {
		return nil, err
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return &ServerMetrics{
		CPUPercent:      cpuPercent,
		MemoryPercent:   percent(mem.Used, mem.Total),
		MemoryTotal:     mem.Total,
		MemoryUsed:      mem.Used,
		MemoryAvailable: mem.Available,
		DiskPercent:     percent(disk.Used, disk.Total),
		DiskTotal:       disk.Total,
		DiskUsed:        disk.Used,
		ProcessMemory:   ms.Alloc,
		Goroutines:      runtime.NumGoroutine(),
		NumCPU:          runtime.NumCPU(),
		DBPools:         collectDatabasePools(),
		CollectedAt:     time.Now(),
	}, nil
}

func (s *ServerMonitorService) GetSetting() (*models.ServerMonitorSetting, error) {
	var setting models.ServerMonitorSetting
	err := s.systemDB.Preload("AlertChannel").First(&setting, 1).Error
	if err == gorm.ErrRecordNotFound {
		setting = models.ServerMonitorSetting{ID: 1, Enabled: true, CPUThreshold: 85, MemoryThreshold: 85, DiskThreshold: 90, GoroutineThreshold: 20000}
		err = s.systemDB.Create(&setting).Error
	}
	return &setting, err
}

func (s *ServerMonitorService) SaveSetting(setting *models.ServerMonitorSetting) error {
	setting.ID = 1
	if setting.CPUThreshold <= 0 {
		setting.CPUThreshold = 85
	}
	if setting.MemoryThreshold <= 0 {
		setting.MemoryThreshold = 85
	}
	if setting.DiskThreshold <= 0 {
		setting.DiskThreshold = 90
	}
	if setting.GoroutineThreshold <= 0 {
		setting.GoroutineThreshold = 20000
	}
	var existing models.ServerMonitorSetting
	err := s.systemDB.First(&existing, 1).Error
	if err == gorm.ErrRecordNotFound {
		return s.systemDB.Create(setting).Error
	}
	if err != nil {
		return err
	}
	return s.systemDB.Model(&models.ServerMonitorSetting{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"enabled":             setting.Enabled,
		"alert_channel_id":    setting.AlertChannelID,
		"cpu_threshold":       setting.CPUThreshold,
		"memory_threshold":    setting.MemoryThreshold,
		"disk_threshold":      setting.DiskThreshold,
		"goroutine_threshold": setting.GoroutineThreshold,
	}).Error
}

func (s *ServerMonitorService) CheckAlerts(ctx context.Context) error {
	setting, err := s.GetSetting()
	if err != nil || !setting.Enabled || setting.AlertChannelID == nil {
		return err
	}
	var channel models.AlertChannel
	if err := s.systemDB.First(&channel, *setting.AlertChannelID).Error; err != nil || channel.Status != 1 {
		return err
	}
	metrics, err := s.Metrics()
	if err != nil {
		return err
	}
	issues := []string{}
	if setting.CPUThreshold > 0 && metrics.CPUPercent >= setting.CPUThreshold {
		issues = append(issues, fmt.Sprintf("CPU %.1f%% >= %.1f%%", metrics.CPUPercent, setting.CPUThreshold))
	}
	if setting.MemoryThreshold > 0 && metrics.MemoryPercent >= setting.MemoryThreshold {
		issues = append(issues, fmt.Sprintf("内存 %.1f%% >= %.1f%%", metrics.MemoryPercent, setting.MemoryThreshold))
	}
	if setting.DiskThreshold > 0 && metrics.DiskPercent >= setting.DiskThreshold {
		issues = append(issues, fmt.Sprintf("磁盘 %.1f%% >= %.1f%%", metrics.DiskPercent, setting.DiskThreshold))
	}
	if setting.GoroutineThreshold > 0 && metrics.Goroutines >= setting.GoroutineThreshold {
		issues = append(issues, fmt.Sprintf("goroutine %d >= %d", metrics.Goroutines, setting.GoroutineThreshold))
	}
	if len(issues) == 0 {
		return nil
	}
	now := time.Now()
	if setting.LastAlertAt != nil && now.Sub(*setting.LastAlertAt) < time.Hour {
		return nil
	}
	content := "服务器性能预警\n" + strings.Join(issues, "\n") + fmt.Sprintf("\n采集时间：%s", metrics.CollectedAt.Format("2006-01-02 15:04:05"))
	if err := s.bot.SendText(ctx, channel.RobotID, content); err != nil {
		return err
	}
	return s.systemDB.Model(&models.ServerMonitorSetting{}).Where("id = ?", 1).Update("last_alert_at", &now).Error
}

func collectDatabasePools() []DatabasePoolStats {
	manager := database.GetManager()
	names := manager.ListConnections()
	stats := make([]DatabasePoolStats, 0, len(names))
	for _, name := range names {
		db, err := manager.GetConnection(name)
		if err != nil {
			continue
		}
		sqlDB, err := db.DB()
		if err != nil {
			continue
		}
		stat := sqlDB.Stats()
		stats = append(stats, DatabasePoolStats{Name: name, Open: stat.OpenConnections, InUse: stat.InUse, Idle: stat.Idle, MaxOpen: stat.MaxOpenConnections, WaitCount: stat.WaitCount, WaitDurationMS: stat.WaitDuration.Milliseconds()})
	}
	return stats
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) * 100 / float64(total)
}

type memorySnapshot struct {
	Total     uint64
	Used      uint64
	Available uint64
}

type diskSnapshot struct {
	Total uint64
	Used  uint64
}
