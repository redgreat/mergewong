package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/redgreat/mergewong/internal/database"
	"github.com/redgreat/mergewong/internal/models"
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
	if err := validateTaskExecutionSettings(task); err != nil {
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
	if err := validateTaskExecutionSettings(task); err != nil {
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
		updates := map[string]interface{}{"source_table": tables[0].SourceTable, "target_table": tables[0].TargetTable, "field_mapping": tables[0].FieldMapping, "validation_status": "pending"}
		return tx.Model(&models.SyncTask{}).Where("id = ?", taskID).Updates(updates).Error
	})
}

var taskIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

func validateTaskTables(tables []models.SyncTaskTable) error {
	if len(tables) == 0 {
		return fmt.Errorf("至少选择一张同步表")
	}
	sources, targets := map[string]bool{}, map[string]bool{}
	for i := range tables {
		table := &tables[i]
		table.SourceTable = strings.TrimSpace(table.SourceTable)
		table.TargetTable = strings.TrimSpace(table.TargetTable)
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
		cleaned, err := normalizeFieldMapping(table.FieldMapping)
		if err != nil {
			return fmt.Errorf("表 %s 字段映射不正确: %w", table.SourceTable, err)
		}
		table.FieldMapping = cleaned
		ignored, err := normalizeIdentifierList(table.IgnoredFields)
		if err != nil {
			return fmt.Errorf("表 %s 忽略字段不正确: %w", table.SourceTable, err)
		}
		table.IgnoredFields = ignored
		confirmed, err := normalizeIdentifierPairList(table.TypeMismatchIgnores)
		if err != nil {
			return fmt.Errorf("表 %s 类型忽略确认不正确: %w", table.SourceTable, err)
		}
		table.TypeMismatchIgnores = confirmed
	}
	return nil
}

func normalizeFieldMapping(mapping models.FieldMapping) (models.FieldMapping, error) {
	if len(mapping) == 0 {
		return models.FieldMapping{}, nil
	}
	cleaned := models.FieldMapping{}
	targets := map[string]string{}
	for source, target := range mapping {
		source = strings.TrimSpace(source)
		target = strings.TrimSpace(target)
		if source == "" && target == "" {
			continue
		}
		if source == "" || target == "" {
			return nil, fmt.Errorf("源字段和目标字段必须同时填写")
		}
		if !taskIdentifierPattern.MatchString(source) || !taskIdentifierPattern.MatchString(target) {
			return nil, fmt.Errorf("字段名只能包含字母、数字、下划线和美元符号，且不能以数字开头")
		}
		if previous, ok := targets[target]; ok && previous != source {
			return nil, fmt.Errorf("目标字段 %s 被多个源字段映射", target)
		}
		if source != target {
			cleaned[source] = target
			targets[target] = source
		}
	}
	return cleaned, nil
}

func normalizeIdentifierList(values models.StringList) (models.StringList, error) {
	cleaned := models.StringList{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !taskIdentifierPattern.MatchString(value) {
			return nil, fmt.Errorf("字段名只能包含字母、数字、下划线和美元符号，且不能以数字开头")
		}
		if !seen[value] {
			cleaned = append(cleaned, value)
			seen[value] = true
		}
	}
	return cleaned, nil
}

func normalizeIdentifierPairList(values models.StringList) (models.StringList, error) {
	cleaned := models.StringList{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts := strings.Split(value, "->")
		if len(parts) != 2 || !taskIdentifierPattern.MatchString(parts[0]) || !taskIdentifierPattern.MatchString(parts[1]) {
			return nil, fmt.Errorf("确认项格式必须为 source->target")
		}
		if !seen[value] {
			cleaned = append(cleaned, value)
			seen[value] = true
		}
	}
	return cleaned, nil
}

func (s *SyncService) ValidateTaskConnections(sourceName, targetName string) error {
	checks := []struct{ name, required string }{{sourceName, "source"}, {targetName, "target"}}
	for _, check := range checks {
		var connection models.DatabaseConnection
		if err := s.systemDB.Where("name = ?", check.name).First(&connection).Error; err != nil {
			return fmt.Errorf("数据库连接 %s 不存在", check.name)
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
	if task.SyncType == "cdc" || task.SyncType == "full_cdc" {
		if task.ScheduleType != "manual" {
			return fmt.Errorf("CDC 任务不需要 Cron 或轮询调度")
		}
	}
	if task.AlertDelaySeconds < 0 {
		return fmt.Errorf("预警时间不能小于 0")
	}
	return nil
}

func validateTaskExecutionSettings(task *models.SyncTask) error {
	if task.SyncBatchSize < 0 {
		return fmt.Errorf("批大小不能小于 0")
	}
	if task.SnapshotTableWorkers < 0 {
		return fmt.Errorf("表并发不能小于 0")
	}
	if task.SnapshotShardWorkers < 0 {
		return fmt.Errorf("分片并发不能小于 0")
	}
	if task.SyncBatchSize > 0 && task.SyncBatchSize < 100 {
		return fmt.Errorf("批大小至少为 100，或填写 0 使用自动配置")
	}
	if task.SyncBatchSize > 20000 {
		return fmt.Errorf("批大小不能超过 20000")
	}
	if task.SnapshotTableWorkers > 32 {
		return fmt.Errorf("表并发不能超过 32")
	}
	if task.SnapshotShardWorkers > 32 {
		return fmt.Errorf("分片并发不能超过 32")
	}
	return nil
}

func (s *SyncService) ValidateTaskExecutionConfig(batchSize, tableWorkers, shardWorkers int) error {
	return validateTaskExecutionSettings(&models.SyncTask{
		SyncBatchSize:        batchSize,
		SnapshotTableWorkers: tableWorkers,
		SnapshotShardWorkers: shardWorkers,
	})
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
	if err := s.systemDB.Order("id DESC").Preload("AlertChannel").Preload("CDCCheckpoint").Preload("TaskTables", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC") }).Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// scheduledRunAction 描述一次定时调度对任务的处理方式。
type scheduledRunAction int

const (
	scheduledRunExecute        scheduledRunAction = iota // 正常执行
	scheduledRunSkipPaused                               // 任务被用户暂停，跳过
	scheduledRunSkipIrrelevant                           // 任务不参与定时调度（CDC 或已禁用）
)

// classifyScheduledRun 判断定时调度应当执行还是跳过（纯函数，便于测试）。
func classifyScheduledRun(task *models.SyncTask) scheduledRunAction {
	if task == nil {
		return scheduledRunSkipIrrelevant
	}
	// CDC / 全量+CDC 任务由常驻 Binlog 链路负责，调度配置被强制为 manual
	if task.SyncType == "cdc" || task.SyncType == "full_cdc" {
		return scheduledRunSkipIrrelevant
	}
	if task.Status == 0 {
		return scheduledRunSkipIrrelevant
	}
	// 用户手动暂停的任务不自动拉起，等待界面上点击"开始"
	if task.RuntimeStatus == "paused" {
		return scheduledRunSkipPaused
	}
	return scheduledRunExecute
}

// ExecuteScheduledTask 定时调度入口，与手动执行 ExecuteTask 的区别：
//  1. 任务被暂停时不自动拉起，避免"暂停"在第二天被调度悄悄恢复；
//  2. 上一次执行还没结束时直接跳过本次调度（由任务级运行锁拦截），
//     不会并发跑第二轮去重复清空目标表或写同一份断点。
//
// 手动执行、界面"开始"和"重置"仍然走 ExecuteTask，行为不变。
func (s *SyncService) ExecuteScheduledTask(taskID uint) error {
	task, err := s.GetTask(taskID)
	if err != nil {
		return err
	}
	switch classifyScheduledRun(task) {
	case scheduledRunSkipIrrelevant:
		return nil
	case scheduledRunSkipPaused:
		return ErrTaskPausedByUser
	}
	return s.ExecuteTask(taskID)
}

// RecordScheduleSkipped 记录一次被跳过的定时调度，便于在任务日志中排查。
func (s *SyncService) RecordScheduleSkipped(taskID uint, reason string) {
	task, err := s.GetTask(taskID)
	if err != nil {
		return
	}
	s.RecordTaskEvent(task, "schedule_skipped", "schedule", "skipped", "定时调度已跳过", reason, 0, 0)
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

	// 全量任务：上一轮已全部跑完时开启新一轮，清空快照断点后重新同步，
	// 保证定时任务每天都会重新做一遍全量，而不是因检查点已完成直接跳过。
	newRound, err := s.beginFullSnapshotRound(task)
	if err != nil {
		return err
	}

	// 更新任务状态为运行中
	now := time.Now()
	runUpdates := map[string]interface{}{
		"last_run_at":      &now,
		"last_run_status":  "running",
		"runtime_status":   "initializing",
		"phase_started_at": &now,
	}
	if newRound {
		runUpdates["rows_processed"] = 0
		runUpdates["rows_per_second"] = 0
		runUpdates["delay_seconds"] = 0
	}
	s.UpdateTask(taskID, runUpdates)
	s.RecordTaskEvent(task, "snapshot_started", "snapshot", "running", "全量数据初始化开始", "", 0, 0)
	if newRound {
		message, detail := "新一轮全量同步开始", "上一轮已全部完成，已清空快照断点，将从第一行重新同步"
		if !task.TruncateBeforeSync {
			message = "新一轮全量同步开始（未清空目标表）"
			detail = "未开启 truncate_before_sync：本轮按主键 upsert 覆盖，源端已删除的行不会从目标表移除；如需与源端完全一致，请开启该选项或使用数据修复"
		}
		s.RecordTaskEvent(task, "snapshot_round_started", "snapshot", "running", message, detail, 0, 0)
	}

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
	_ = alertService.ResolveTaskAlertSilent(taskID, "error")
	_ = alertService.ResolveTaskAlertSilent(taskID, "delay")

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
func (s *SyncService) GetTaskLogs(taskID uint, page, pageSize int, fromTime, toTime *time.Time) ([]models.SyncLog, int64, error) {
	var logs []models.SyncLog
	var total int64

	query := s.systemDB.Model(&models.SyncLog{})
	if taskID > 0 {
		query = query.Where("task_id = ?", taskID)
	}
	if fromTime != nil {
		query = query.Where("created_at >= ?", *fromTime)
	}
	if toTime != nil {
		query = query.Where("created_at <= ?", *toTime)
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
