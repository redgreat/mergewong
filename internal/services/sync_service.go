package services

import (
	"context"
	"errors"
	"fmt"
	stdlog "log"
	"regexp"
	"time"

	"github.com/redgreat/apiwong/internal/database"
	"github.com/redgreat/apiwong/internal/models"
	"gorm.io/gorm"
)

// SyncService 同步服务
type SyncService struct {
	systemDB *gorm.DB
}

// NewSyncService 创建同步服务
func NewSyncService() *SyncService {
	db, _ := database.GetManager().GetConnection("system")
	return &SyncService{systemDB: db}
}

// CreateTask 创建同步任务
func (s *SyncService) CreateTask(task *models.SyncTask) error {
	if err := s.ValidateTaskConnections(task.SourceDB, task.TargetDB); err != nil {
		return err
	}
	if err := s.validateAlertChannel(task.AlertChannelID); err != nil {
		return err
	}
	if err := validateTaskAlertSettings(task); err != nil {
		return err
	}
	return s.systemDB.Create(task).Error
}

func (s *SyncService) CreateTaskWithTables(task *models.SyncTask, tables []models.SyncTaskTable) error {
	if err := validateTaskTables(tables); err != nil {
		return err
	}
	first := tables[0]
	task.SourceTable, task.TargetTable = first.SourceTable, first.TargetTable
	if first.FieldMapping != nil {
		task.FieldMapping = first.FieldMapping
	}
	task.Status = 0
	task.ValidationStatus = "pending"
	task.RuntimeStatus = "pending"
	if err := s.ValidateTaskConnections(task.SourceDB, task.TargetDB); err != nil {
		return err
	}
	if err := s.validateAlertChannel(task.AlertChannelID); err != nil {
		return err
	}
	if err := validateTaskAlertSettings(task); err != nil {
		return err
	}
	return s.systemDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		for i := range tables {
			tables[i].TaskID = task.ID
			tables[i].Position = i
		}
		return tx.Create(&tables).Error
	})
}

func (s *SyncService) ReplaceTaskTables(taskID uint, tables []models.SyncTaskTable) error {
	if err := validateTaskTables(tables); err != nil {
		return err
	}
	return s.systemDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", taskID).Delete(&models.SyncCDCCheckpoint{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_table_id IN (?)", tx.Model(&models.SyncTaskTable{}).Select("id").Where("task_id = ?", taskID)).Delete(&models.SyncCheckpoint{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", taskID).Delete(&models.SyncTaskTable{}).Error; err != nil {
			return err
		}
		for i := range tables {
			tables[i].TaskID, tables[i].Position = taskID, i
		}
		if err := tx.Create(&tables).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{"source_table": tables[0].SourceTable, "target_table": tables[0].TargetTable, "status": 0, "validation_status": "pending", "runtime_status": "pending"}
		return tx.Model(&models.SyncTask{}).Where("id = ?", taskID).Updates(updates).Error
	})
}

var taskIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

func validateTaskTables(tables []models.SyncTaskTable) error {
	if len(tables) == 0 {
		return fmt.Errorf("至少选择一张同步表")
	}
	sources, targets := map[string]bool{}, map[string]bool{}
	for _, table := range tables {
		if !taskIdentifierPattern.MatchString(table.SourceTable) || !taskIdentifierPattern.MatchString(table.TargetTable) {
			return fmt.Errorf("表名只能包含字母、数字、下划线和美元符号，且不能以数字开头")
		}
		if sources[table.SourceTable] {
			return fmt.Errorf("源表 %s 重复选择", table.SourceTable)
		}
		if targets[table.TargetTable] {
			return fmt.Errorf("目标表 %s 重复", table.TargetTable)
		}
		sources[table.SourceTable], targets[table.TargetTable] = true, true
	}
	return nil
}

func (s *SyncService) ValidateTaskConnections(sourceName, targetName string) error {
	checks := []struct{ name, required string }{{sourceName, "source"}, {targetName, "target"}}
	for _, check := range checks {
		var connection models.DatabaseConnection
		if err := s.systemDB.Where("name = ? AND status = ?", check.name, 1).First(&connection).Error; err != nil {
			return fmt.Errorf("数据库连接 %s 不存在或已禁用", check.name)
		}
		if connection.Usage != "both" && connection.Usage != check.required {
			label := "源端"
			if check.required == "target" {
				label = "目标端"
			}
			return fmt.Errorf("数据库连接 %s 不能用作%s", check.name, label)
		}
	}
	return nil
}

func validateTaskAlertSettings(task *models.SyncTask) error {
	if task.SyncType != "full" && task.SyncType != "cdc" && task.SyncType != "full_cdc" {
		return fmt.Errorf("不支持的同步类型")
	}
	if task.ScheduleType != "manual" {
		return fmt.Errorf("全量初始化和 Binlog CDC 不需要 Cron 或轮询调度")
	}
	if task.AlertDelayMinutes < 0 || task.AlertStoppedMinutes < 0 {
		return fmt.Errorf("预警时间不能小于 0")
	}
	if task.ScheduleType == "interval" && task.AlertStoppedMinutes > 0 && task.AlertStoppedMinutes <= task.IntervalMinutes {
		return fmt.Errorf("停止阈值必须大于任务执行间隔")
	}
	if task.AlertChannelID != nil && task.AlertDelayMinutes == 0 && task.AlertStoppedMinutes == 0 && !task.AlertOnError {
		return fmt.Errorf("选择预警发送方后至少启用一种预警")
	}
	if task.AlertCooldownMinutes <= 0 {
		task.AlertCooldownMinutes = 30
	}
	return nil
}

func (s *SyncService) ValidateAlertChannelID(id uint) error {
	if id == 0 {
		return nil
	}
	return s.validateAlertChannel(&id)
}

func (s *SyncService) validateAlertChannel(id *uint) error {
	if id == nil {
		return nil
	}
	var count int64
	if err := s.systemDB.Model(&models.AlertChannel{}).Where("id = ? AND status = ?", *id, 1).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("预警发送方不存在或已禁用")
	}
	return nil
}

// GetTask 获取同步任务
func (s *SyncService) GetTask(id uint) (*models.SyncTask, error) {
	var task models.SyncTask
	if err := s.systemDB.Preload("AlertChannel").Preload("CDCCheckpoint").Preload("TaskTables", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC") }).First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTask 更新同步任务
func (s *SyncService) UpdateTask(id uint, updates map[string]interface{}) error {
	return s.systemDB.Model(&models.SyncTask{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteTask 删除同步任务
func (s *SyncService) DeleteTask(id uint) error {
	return s.systemDB.Delete(&models.SyncTask{}, id).Error
}

// ListTasks 列出所有同步任务
func (s *SyncService) ListTasks(page, pageSize int) ([]models.SyncTask, int64, error) {
	var tasks []models.SyncTask
	var total int64

	s.systemDB.Model(&models.SyncTask{}).Count(&total)

	offset := (page - 1) * pageSize
	if err := s.systemDB.Preload("AlertChannel").Preload("CDCCheckpoint").Preload("TaskTables", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC") }).Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// ExecuteTask 执行同步任务
func (s *SyncService) ExecuteTask(taskID uint) error {
	task, err := s.GetTask(taskID)
	if err != nil {
		return err
	}
	if task.SyncType == "cdc" || task.SyncType == "full_cdc" {
		return GetCDCManager().StartTask(taskID)
	}
	release, err := acquireTaskRunLock(taskID)
	if err != nil {
		return err
	}
	defer release()
	// 获取任务
	// 检查任务状态
	if task.Status == 0 {
		return fmt.Errorf("任务已禁用")
	}
	if task.ValidationStatus == "pending" || task.ValidationStatus == "failed" {
		return fmt.Errorf("任务预检查尚未通过")
	}
	if err := s.ValidateTaskConnections(task.SourceDB, task.TargetDB); err != nil {
		return err
	}

	// 更新任务状态为运行中
	now := time.Now()
	s.UpdateTask(taskID, map[string]interface{}{
		"last_run_at":      &now,
		"last_run_status":  "running",
		"runtime_status":   "initializing",
		"phase_started_at": &now,
	})
	s.RecordTaskEvent(task, "snapshot_started", "snapshot", "running", "全量数据初始化开始", "", 0, 0)

	// 创建同步日志
	log := &models.SyncLog{
		TaskID: taskID, TaskName: task.Name, EventType: "snapshot_run", Phase: "snapshot",
		Status: "running", CreatedAt: now,
	}

	startTime := time.Now()

	// 执行同步
	var rowsAffected int64
	if task.ValidationStatus == "passed" && len(task.TaskTables) > 0 {
		rowsAffected, err = s.syncValidatedTask(task)
	} else {
		rowsAffected, err = s.syncData(task)
	}
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		if errors.Is(err, ErrTaskPaused) {
			log.Status, log.Message, log.Duration = "success", "全量初始化已暂停", duration
			s.systemDB.Create(log)
			s.UpdateTask(taskID, map[string]interface{}{"last_run_status": "paused", "runtime_status": "paused", "last_run_message": "全量初始化已暂停"})
			return nil
		}
		// 同步失败
		log.Status = "failed"
		log.Message = "同步失败"
		log.ErrorDetail = err.Error()
		log.Duration = duration

		if task.AlertOnError && task.AlertChannel != nil && task.AlertChannel.Status == 1 {
			content := fmt.Sprintf("MergeWong 同步任务预警\n任务：%s\n状态：执行失败\n时间：%s\n原因：%s", task.Name, time.Now().Format("2006-01-02 15:04:05"), err.Error())
			alertCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			alertErr := NewAlertService().SendTaskAlert(alertCtx, task, "error", content)
			cancel()
			if alertErr != nil {
				stdlog.Printf("任务 %d 预警发送失败: %v", taskID, alertErr)
				log.ErrorDetail += "; 预警发送失败: " + alertErr.Error()
			}
		}

		s.systemDB.Create(log)
		s.UpdateTask(taskID, map[string]interface{}{
			"last_run_status":  "failed",
			"runtime_status":   "failed",
			"last_run_message": err.Error(),
		})
		return err
	}

	// 同步成功
	log.Status = "success"
	log.Message = "同步成功"
	log.RowsAffected = rowsAffected
	log.Duration = duration

	s.systemDB.Create(log)
	s.UpdateTask(taskID, map[string]interface{}{
		"last_run_status":  "success",
		"runtime_status":   "completed",
		"last_run_message": fmt.Sprintf("成功同步 %d 行数据", rowsAffected),
		"last_success_at":  time.Now(),
		"rows_processed":   rowsAffected,
		"rows_per_second": func() float64 {
			if duration > 0 {
				return float64(rowsAffected) / (float64(duration) / 1000)
			}
			return 0
		}(),
	})
	s.RecordTaskEvent(task, "snapshot_completed", "snapshot", "success", "全量数据初始化完成", "", rowsAffected, duration)
	alertService := NewAlertService()
	_ = alertService.ResolveTaskAlert(taskID, "error")
	_ = alertService.ResolveTaskAlert(taskID, "delay")

	return nil
}

// syncData 执行数据同步
func (s *SyncService) syncData(task *models.SyncTask) (int64, error) {
	// 获取源数据库连接
	sourceDB, err := database.GetManager().GetConnection(task.SourceDB)
	if err != nil {
		return 0, fmt.Errorf("获取源数据库连接失败: %w", err)
	}

	// 获取目标数据库连接
	targetDB, err := database.GetManager().GetConnection(task.TargetDB)
	if err != nil {
		return 0, fmt.Errorf("获取目标数据库连接失败: %w", err)
	}

	// 构建查询 SQL
	var querySQL string
	var params []interface{}

	if task.SyncType == "full" {
		// 全量同步
		querySQL = fmt.Sprintf("SELECT * FROM %s", task.SourceTable)
	}

	// 查询源数据
	rows, err := sourceDB.Raw(querySQL, params...).Rows()
	if err != nil {
		return 0, fmt.Errorf("查询源数据失败: %w", err)
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("获取列名失败: %w", err)
	}

	var totalRows int64 = 0

	// 开始事务
	tx := targetDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 读取并插入数据
	for rows.Next() {
		// 创建接收数据的切片
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// 扫描数据
		if err := rows.Scan(valuePtrs...); err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("扫描数据失败: %w", err)
		}

		// 转换为 map
		data := make(map[string]interface{})
		for i, col := range columns {
			// 应用字段映射
			targetCol := col
			if mappedCol, ok := task.FieldMapping[col]; ok {
				targetCol = mappedCol
			}

			data[targetCol] = values[i]
		}

		// 插入目标表
		if err := tx.Table(task.TargetTable).Create(data).Error; err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("插入数据失败: %w", err)
		}

		totalRows++
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("提交事务失败: %w", err)
	}

	return totalRows, nil
}

// GetTaskLogs 获取任务日志
func (s *SyncService) GetTaskLogs(taskID uint, page, pageSize int) ([]models.SyncLog, int64, error) {
	var logs []models.SyncLog
	var total int64

	query := s.systemDB.Model(&models.SyncLog{})
	if taskID > 0 {
		query = query.Where("task_id = ?", taskID)
	}
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
