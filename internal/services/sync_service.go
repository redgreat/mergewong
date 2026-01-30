package services

import (
	"fmt"
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
	return s.systemDB.Create(task).Error
}

// GetTask 获取同步任务
func (s *SyncService) GetTask(id uint) (*models.SyncTask, error) {
	var task models.SyncTask
	if err := s.systemDB.First(&task, id).Error; err != nil {
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
	if err := s.systemDB.Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// ExecuteTask 执行同步任务
func (s *SyncService) ExecuteTask(taskID uint) error {
	// 获取任务
	task, err := s.GetTask(taskID)
	if err != nil {
		return err
	}

	// 检查任务状态
	if task.Status == 0 {
		return fmt.Errorf("任务已禁用")
	}

	// 更新任务状态为运行中
	now := time.Now()
	s.UpdateTask(taskID, map[string]interface{}{
		"last_run_at":     &now,
		"last_run_status": "running",
	})

	// 创建同步日志
	log := &models.SyncLog{
		TaskID:    taskID,
		Status:    "running",
		CreatedAt: now,
	}

	startTime := time.Now()

	// 执行同步
	rowsAffected, err := s.syncData(task)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		// 同步失败
		log.Status = "failed"
		log.Message = "同步失败"
		log.ErrorDetail = err.Error()
		log.Duration = duration

		s.systemDB.Create(log)
		s.UpdateTask(taskID, map[string]interface{}{
			"last_run_status":  "failed",
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
		"last_run_message": fmt.Sprintf("成功同步 %d 行数据", rowsAffected),
	})

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
	} else if task.SyncType == "incremental" {
		// 增量同步
		if task.IncrementalKey == "" {
			return 0, fmt.Errorf("增量同步需要指定增量关键字段")
		}

		// 获取目标表中的最大值
		var maxValue interface{}
		targetDB.Table(task.TargetTable).Select(fmt.Sprintf("MAX(%s)", task.IncrementalKey)).Scan(&maxValue)

		if maxValue != nil {
			querySQL = fmt.Sprintf("SELECT * FROM %s WHERE %s > ?", task.SourceTable, task.IncrementalKey)
			params = append(params, maxValue)
		} else {
			querySQL = fmt.Sprintf("SELECT * FROM %s", task.SourceTable)
		}
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

	s.systemDB.Model(&models.SyncLog{}).Where("task_id = ?", taskID).Count(&total)

	offset := (page - 1) * pageSize
	if err := s.systemDB.Where("task_id = ?", taskID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
