package handlers

import (
	"strconv"
	"strings"
	"time"

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
	AlertDelaySeconds    int                `json:"alert_delay_seconds"`
	AlertDelayMS         int                `json:"alert_delay_ms"`
	AlertStoppedMinutes  int                `json:"alert_stopped_minutes"`
	AlertOnError         bool               `json:"alert_on_error"`
	AlertCooldownMinutes int                `json:"alert_cooldown_minutes"`
}

type TaskTableRequest struct {
	SourceTable         string            `json:"source_table" binding:"required"`
	TargetTable         string            `json:"target_table" binding:"required"`
	FieldMapping        map[string]string `json:"field_mapping"`
	IgnoredFields       []string          `json:"ignored_fields"`
	TypeMismatchIgnores []string          `json:"type_mismatch_ignores"`
}

// CreateTask 创建同步任务
func (h *SyncHandler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	alertDelaySeconds := req.AlertDelaySeconds
	if req.AlertDelayMS > 0 {
		alertDelaySeconds = (req.AlertDelayMS + 999) / 1000
	}
	alertChannelID := normalizeAlertChannelID(req.AlertChannelID)
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
		AlertChannelID:       alertChannelID,
		AlertDelaySeconds:    alertDelaySeconds,
		AlertStoppedMinutes:  0,
		AlertOnError:         true,
		AlertCooldownMinutes: 0,
		Status:               1,
		UserID:               userID.(uint),
	}

	tableRequests := req.Tables
	if len(tableRequests) == 0 && req.SourceTable != "" && req.TargetTable != "" {
		tableRequests = []TaskTableRequest{{SourceTable: req.SourceTable, TargetTable: req.TargetTable, FieldMapping: req.FieldMapping}}
	}
	tables := make([]models.SyncTaskTable, 0, len(tableRequests))
	for _, table := range tableRequests {
		tables = append(tables, models.SyncTaskTable{SourceTable: table.SourceTable, TargetTable: table.TargetTable, FieldMapping: table.FieldMapping, IgnoredFields: table.IgnoredFields, TypeMismatchIgnores: table.TypeMismatchIgnores})
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
	AlertChannelID       *uint              `json:"alert_channel_id"`
	AlertDelaySeconds    int                `json:"alert_delay_seconds"`
	AlertDelayMS         int                `json:"alert_delay_ms"`
	AlertStoppedMinutes  *int               `json:"alert_stopped_minutes"`
	AlertOnError         *bool              `json:"alert_on_error"`
	AlertCooldownMinutes *int               `json:"alert_cooldown_minutes"`
	Tables               []TaskTableRequest `json:"tables"`
}

// UpdateTask 更新任务（仅允许修改同步对象和预警策略）
func (h *SyncHandler) UpdateTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	currentTask, err := h.syncService.GetTask(uint(id))
	if err != nil {
		utils.Error(c, 404, "任务不存在")
		return
	}
	running := currentTask.RuntimeStatus == "initializing" || currentTask.RuntimeStatus == "catching_up" || currentTask.RuntimeStatus == "cdc_running"

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	alertDelaySeconds := req.AlertDelaySeconds
	if req.AlertDelayMS > 0 {
		alertDelaySeconds = (req.AlertDelayMS + 999) / 1000
	}
	if alertDelaySeconds < 0 {
		utils.BadRequest(c, "预警时间配置不正确")
		return
	}
	alertChannelID := normalizeAlertChannelID(req.AlertChannelID)
	channelID := uint(0)
	if alertChannelID != nil {
		channelID = *alertChannelID
	}
	if err := h.syncService.ValidateAlertChannelID(channelID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	updates := map[string]interface{}{
		"alert_delay_seconds":    alertDelaySeconds,
		"alert_stopped_minutes":  0,
		"alert_on_error":         true,
		"alert_cooldown_minutes": 0,
	}
	if alertChannelID == nil {
		updates["alert_channel_id"] = nil
	} else {
		updates["alert_channel_id"] = *alertChannelID
	}

	if err := h.syncService.UpdateTask(uint(id), updates); err != nil {
		utils.InternalServerError(c, "更新任务失败: "+err.Error())
		return
	}
	if len(req.Tables) > 0 {
		tables := make([]models.SyncTaskTable, 0, len(req.Tables))
		for _, table := range req.Tables {
			tables = append(tables, models.SyncTaskTable{SourceTable: table.SourceTable, TargetTable: table.TargetTable, FieldMapping: table.FieldMapping, IgnoredFields: table.IgnoredFields, TypeMismatchIgnores: table.TypeMismatchIgnores})
		}
		var tableErr error
		if running {
			_, tableErr = h.syncService.AddTaskTablesOnline(uint(id), tables)
		} else {
			services.GetCDCManager().StopTask(uint(id))
			tableErr = h.syncService.ReplaceTaskTables(uint(id), tables)
		}
		if tableErr != nil {
			utils.InternalServerError(c, "更新同步对象失败: "+tableErr.Error())
			return
		}
	}
	updatedTask, err := h.syncService.GetTask(uint(id))
	if err != nil || scheduler.GetScheduler().RefreshTask(updatedTask) != nil {
		utils.InternalServerError(c, "任务已保存，但刷新调度失败")
		return
	}

	utils.SuccessWithMessage(c, "更新成功", gin.H{"online_onboarding": running})
	h.syncService.RecordTaskEvent(updatedTask, "task_updated", "config", "success", "同步任务配置已修改", "", 0, 0)
}

func normalizeAlertChannelID(id *uint) *uint {
	if id == nil || *id == 0 {
		return nil
	}
	return id
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

type RepairCompareRequest struct {
	CutoffTime   string `json:"cutoff_time"`
	CutoffColumn string `json:"cutoff_column"`
}

func (h *SyncHandler) ListRepairJobs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	jobs, err := services.NewRepairService().ListJobs(uint(id))
	if err != nil {
		utils.InternalServerError(c, "获取数据修复任务失败: "+err.Error())
		return
	}
	utils.Success(c, jobs)
}

func (h *SyncHandler) StartRepairCompare(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req RepairCompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	compareReq := services.RepairCompareRequest{CutoffColumn: strings.TrimSpace(req.CutoffColumn)}
	if strings.TrimSpace(req.CutoffTime) != "" {
		cutoff, err := parseRepairTime(req.CutoffTime)
		if err != nil {
			utils.BadRequest(c, "截止时间格式错误")
			return
		}
		compareReq.CutoffTime = &cutoff
	}
	job, err := services.NewRepairService().StartCompare(uint(id), compareReq)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessWithMessage(c, "全量对比已开始", job)
}

func (h *SyncHandler) StartRepairApply(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	jobID, _ := strconv.ParseUint(c.Param("job_id"), 10, 32)
	job, err := services.NewRepairService().StartRepair(uint(id), uint(jobID))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessWithMessage(c, "补数已开始", job)
}

func (h *SyncHandler) CancelRepairJob(c *gin.Context) {
	jobID, _ := strconv.ParseUint(c.Param("job_id"), 10, 32)
	if err := services.NewRepairService().CancelJob(uint(jobID)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessWithMessage(c, "已取消", nil)
}

func (h *SyncHandler) ListRepairDiffs(c *gin.Context) {
	jobID, _ := strconv.ParseUint(c.Param("job_id"), 10, 32)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	diffs, total, err := services.NewRepairService().ListDiffs(uint(jobID), page, pageSize)
	if err != nil {
		utils.InternalServerError(c, "获取差异失败: "+err.Error())
		return
	}
	utils.Success(c, gin.H{"data": diffs, "total": total, "page": page, "page_size": pageSize})
}

func parseRepairTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04", value, time.Local); err == nil {
		return parsed, nil
	}
	return time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
}
