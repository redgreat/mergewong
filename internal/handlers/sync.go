package handlers

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redgreat/mergewong/internal/models"
	"github.com/redgreat/mergewong/internal/scheduler"
	"github.com/redgreat/mergewong/internal/services"
	"github.com/redgreat/mergewong/internal/utils"
)

// SyncHandler 同步处理器
type SyncHandler struct {
	syncService *services.SyncService
}

// NewSyncHandler 创建同步处理器
func NewSyncHandler() *SyncHandler {
	return &SyncHandler{
		syncService: services.NewSyncService(),
	}
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Name                 string             `json:"name" binding:"required"`
	SourceDB             string             `json:"source_db" binding:"required"`
	SourceTable          string             `json:"source_table"`
	TargetDB             string             `json:"target_db" binding:"required"`
	TargetTable          string             `json:"target_table"`
	Tables               []TaskTableRequest `json:"tables"`
	FieldMapping         map[string]string  `json:"field_mapping"`
	SyncType             string             `json:"sync_type" binding:"required,oneof=full cdc full_cdc"`
	CronExpression       string             `json:"cron_expression"`
	ScheduleType         string             `json:"schedule_type" binding:"required,oneof=manual interval cron"`
	IntervalMinutes      int                `json:"interval_minutes"`
	AlertChannelID       *uint              `json:"alert_channel_id"`
	AlertDelayMinutes    int                `json:"alert_delay_minutes"`
	AlertStoppedMinutes  int                `json:"alert_stopped_minutes"`
	AlertOnError         *bool              `json:"alert_on_error"`
	AlertCooldownMinutes int                `json:"alert_cooldown_minutes"`
}

type TaskTableRequest struct {
	SourceTable  string            `json:"source_table" binding:"required"`
	TargetTable  string            `json:"target_table" binding:"required"`
	FieldMapping map[string]string `json:"field_mapping"`
}

// CreateTask 创建同步任务
func (h *SyncHandler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	alertOnError := true
	if req.AlertOnError != nil {
		alertOnError = *req.AlertOnError
	}

	task := &models.SyncTask{
		Name:                 req.Name,
		SourceDB:             req.SourceDB,
		SourceTable:          req.SourceTable,
		TargetDB:             req.TargetDB,
		TargetTable:          req.TargetTable,
		FieldMapping:         req.FieldMapping,
		SyncType:             req.SyncType,
		CronExpression:       req.CronExpression,
		ScheduleType:         req.ScheduleType,
		IntervalMinutes:      req.IntervalMinutes,
		AlertChannelID:       req.AlertChannelID,
		AlertDelayMinutes:    req.AlertDelayMinutes,
		AlertStoppedMinutes:  req.AlertStoppedMinutes,
		AlertOnError:         alertOnError,
		AlertCooldownMinutes: req.AlertCooldownMinutes,
		Status:               1,
		UserID:               userID.(uint),
	}

	tableRequests := req.Tables
	if len(tableRequests) == 0 && req.SourceTable != "" && req.TargetTable != "" {
		tableRequests = []TaskTableRequest{{SourceTable: req.SourceTable, TargetTable: req.TargetTable, FieldMapping: req.FieldMapping}}
	}
	tables := make([]models.SyncTaskTable, 0, len(tableRequests))
	for _, table := range tableRequests {
		tables = append(tables, models.SyncTaskTable{SourceTable: table.SourceTable, TargetTable: table.TargetTable, FieldMapping: table.FieldMapping})
	}
	if err := h.syncService.CreateTaskWithTables(task, tables); err != nil {
		utils.InternalServerError(c, "创建任务失败: "+err.Error())
		return
	}
	if err := scheduler.GetScheduler().RefreshTask(task); err != nil {
		_ = h.syncService.DeleteTask(task.ID)
		utils.BadRequest(c, "调度配置错误: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "创建成功", task)
	h.syncService.RecordTaskEvent(task, "task_created", "config", "success", "同步任务已创建", "", 0, 0)
}

// ListTasks 列出所有任务
func (h *SyncHandler) ListTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	tasks, total, err := h.syncService.ListTasks(page, pageSize)
	if err != nil {
		utils.InternalServerError(c, "获取任务列表失败: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"data":      tasks,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetTask 获取任务详情
func (h *SyncHandler) GetTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	task, err := h.syncService.GetTask(uint(id))
	if err != nil {
		utils.Error(c, 404, "任务不存在")
		return
	}

	utils.Success(c, task)
}

// UpdateTaskRequest 更新任务请求
type UpdateTaskRequest struct {
	Name                 string             `json:"name"`
	SourceDB             string             `json:"source_db"`
	SourceTable          string             `json:"source_table"`
	TargetDB             string             `json:"target_db"`
	TargetTable          string             `json:"target_table"`
	FieldMapping         map[string]string  `json:"field_mapping"`
	SyncType             string             `json:"sync_type"`
	CronExpression       string             `json:"cron_expression"`
	ScheduleType         string             `json:"schedule_type"`
	IntervalMinutes      int                `json:"interval_minutes"`
	Status               *int               `json:"status"`
	AlertChannelID       uint               `json:"alert_channel_id"`
	AlertDelayMinutes    int                `json:"alert_delay_minutes"`
	AlertStoppedMinutes  int                `json:"alert_stopped_minutes"`
	AlertOnError         *bool              `json:"alert_on_error"`
	AlertCooldownMinutes int                `json:"alert_cooldown_minutes"`
	Tables               []TaskTableRequest `json:"tables"`
}

// UpdateTask 更新任务
func (h *SyncHandler) UpdateTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	services.GetCDCManager().StopTask(uint(id))

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	if req.AlertDelayMinutes < 0 || req.AlertStoppedMinutes < 0 || req.AlertCooldownMinutes < 0 {
		utils.BadRequest(c, "预警时间配置不正确")
		return
	}
	if req.ScheduleType != "" && req.ScheduleType != "manual" {
		utils.BadRequest(c, "全量初始化和 Binlog CDC 不需要 Cron 或轮询调度")
		return
	}
	if req.ScheduleType == "interval" && req.AlertStoppedMinutes > 0 && req.AlertStoppedMinutes <= req.IntervalMinutes {
		utils.BadRequest(c, "停止阈值必须大于任务执行间隔")
		return
	}
	if req.ScheduleType != "" {
		candidate := &models.SyncTask{ScheduleType: req.ScheduleType, IntervalMinutes: req.IntervalMinutes, CronExpression: req.CronExpression}
		if _, err := scheduler.ScheduleSpec(candidate); err != nil && req.ScheduleType != "manual" {
			utils.BadRequest(c, "调度配置错误: "+err.Error())
			return
		}
	}
	if req.SourceDB != "" && req.TargetDB != "" {
		if err := h.syncService.ValidateTaskConnections(req.SourceDB, req.TargetDB); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.SourceDB != "" {
		updates["source_db"] = req.SourceDB
	}
	if req.SourceTable != "" {
		updates["source_table"] = req.SourceTable
	}
	if req.TargetDB != "" {
		updates["target_db"] = req.TargetDB
	}
	if req.TargetTable != "" {
		updates["target_table"] = req.TargetTable
	}
	if req.FieldMapping != nil {
		updates["field_mapping"] = req.FieldMapping
	}
	if req.SyncType != "" {
		updates["sync_type"] = req.SyncType
	}
	if req.CronExpression != "" {
		updates["cron_expression"] = req.CronExpression
	}
	if req.ScheduleType != "" {
		updates["schedule_type"] = req.ScheduleType
	}
	updates["interval_minutes"] = req.IntervalMinutes
	updates["alert_delay_minutes"] = req.AlertDelayMinutes
	updates["alert_stopped_minutes"] = req.AlertStoppedMinutes
	if req.AlertCooldownMinutes > 0 {
		updates["alert_cooldown_minutes"] = req.AlertCooldownMinutes
	}
	if req.AlertOnError != nil {
		updates["alert_on_error"] = *req.AlertOnError
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if err := h.syncService.ValidateAlertChannelID(req.AlertChannelID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if req.AlertChannelID == 0 {
		updates["alert_channel_id"] = nil
	} else {
		updates["alert_channel_id"] = req.AlertChannelID
	}

	if err := h.syncService.UpdateTask(uint(id), updates); err != nil {
		utils.InternalServerError(c, "更新任务失败: "+err.Error())
		return
	}
	if len(req.Tables) > 0 {
		tables := make([]models.SyncTaskTable, 0, len(req.Tables))
		for _, table := range req.Tables {
			tables = append(tables, models.SyncTaskTable{SourceTable: table.SourceTable, TargetTable: table.TargetTable, FieldMapping: table.FieldMapping})
		}
		if err := h.syncService.ReplaceTaskTables(uint(id), tables); err != nil {
			utils.InternalServerError(c, "更新同步对象失败: "+err.Error())
			return
		}
	}
	updatedTask, err := h.syncService.GetTask(uint(id))
	if err != nil || scheduler.GetScheduler().RefreshTask(updatedTask) != nil {
		utils.InternalServerError(c, "任务已保存，但刷新调度失败")
		return
	}

	utils.SuccessWithMessage(c, "更新成功", nil)
	h.syncService.RecordTaskEvent(updatedTask, "task_updated", "config", "success", "同步任务配置已修改", "", 0, 0)
}

// DeleteTask 删除任务
func (h *SyncHandler) DeleteTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	task, err := h.syncService.GetTask(uint(id))
	if err != nil {
		utils.Error(c, 404, "任务不存在")
		return
	}
	if task.RuntimeStatus != "paused" && task.RuntimeStatus != "stopped" && task.RuntimeStatus != "completed" {
		utils.BadRequest(c, "任务必须暂停、停止或完成后才能删除")
		return
	}

	services.GetCDCManager().StopTask(uint(id))
	if err := h.syncService.DeleteTask(uint(id)); err != nil {
		utils.InternalServerError(c, "删除任务失败: "+err.Error())
		return
	}
	scheduler.GetScheduler().RemoveTask(uint(id))
	h.syncService.RecordTaskEvent(task, "task_deleted", "config", "success", "同步任务已删除", "", 0, 0)

	utils.SuccessWithMessage(c, "删除成功", nil)
}

func (h *SyncHandler) PauseTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.syncService.PauseTask(uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessWithMessage(c, "任务已暂停", nil)
}

func (h *SyncHandler) ResumeTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.syncService.ResumeTask(uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessWithMessage(c, "任务已开始", nil)
}

func (h *SyncHandler) UpdateCheckpoint(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		File     string `json:"file" binding:"required"`
		Position uint32 `json:"position" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写 file 和 position")
		return
	}
	if err := h.syncService.UpdateBinlogPosition(uint(id), strings.TrimSpace(req.File), req.Position); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessWithMessage(c, "Binlog 位点已修改", nil)
}

// ExecuteTask 手动执行任务
func (h *SyncHandler) ExecuteTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// 异步执行
	go h.syncService.ExecuteTask(uint(id))

	utils.SuccessWithMessage(c, "任务已开始执行", nil)
}

func (h *SyncHandler) PrecheckTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	result, err := h.syncService.PrecheckTask(uint(id))
	if err != nil {
		utils.InternalServerError(c, "预检查失败: "+err.Error())
		return
	}
	if result.Passed {
		task, err := h.syncService.GetTask(uint(id))
		if err == nil {
			_ = scheduler.GetScheduler().RefreshTask(task)
			if task.SyncType == "cdc" || task.SyncType == "full_cdc" {
				_ = services.GetCDCManager().StartTask(task.ID)
			}
		}
	}
	if task, taskErr := h.syncService.GetTask(uint(id)); taskErr == nil {
		status := "failed"
		message := "任务预检查未通过"
		if result.Passed {
			status = "success"
			message = "任务预检查通过"
		}
		h.syncService.RecordTaskEvent(task, "precheck", "precheck", status, message, "", 0, 0)
	}
	utils.Success(c, result)
}

// GetTaskLogs 获取任务日志
func (h *SyncHandler) GetTaskLogs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	logs, total, err := h.syncService.GetTaskLogs(uint(id), page, pageSize)
	if err != nil {
		utils.InternalServerError(c, "获取日志失败: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *SyncHandler) ListLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	taskID, _ := strconv.ParseUint(c.DefaultQuery("task_id", "0"), 10, 32)
	logs, total, err := h.syncService.GetTaskLogs(uint(taskID), page, pageSize)
	if err != nil {
		utils.InternalServerError(c, "获取日志失败: "+err.Error())
		return
	}
	utils.Success(c, gin.H{"data": logs, "total": total, "page": page, "page_size": pageSize})
}
