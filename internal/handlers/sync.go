package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redgreat/apiwong/internal/models"
	"github.com/redgreat/apiwong/internal/services"
	"github.com/redgreat/apiwong/internal/utils"
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
	Name           string                 `json:"name" binding:"required"`
	SourceDB       string                 `json:"source_db" binding:"required"`
	SourceTable    string                 `json:"source_table" binding:"required"`
	TargetDB       string                 `json:"target_db" binding:"required"`
	TargetTable    string                 `json:"target_table" binding:"required"`
	FieldMapping   map[string]string      `json:"field_mapping"`
	SyncType       string                 `json:"sync_type" binding:"required,oneof=full incremental"`
	IncrementalKey string                 `json:"incremental_key"`
	CronExpression string                 `json:"cron_expression"`
}

// CreateTask 创建同步任务
func (h *SyncHandler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")

	task := &models.SyncTask{
		Name:           req.Name,
		SourceDB:       req.SourceDB,
		SourceTable:    req.SourceTable,
		TargetDB:       req.TargetDB,
		TargetTable:    req.TargetTable,
		FieldMapping:   req.FieldMapping,
		SyncType:       req.SyncType,
		IncrementalKey: req.IncrementalKey,
		CronExpression: req.CronExpression,
		Status:         1,
		UserID:         userID.(uint),
	}

	if err := h.syncService.CreateTask(task); err != nil {
		utils.InternalServerError(c, "创建任务失败: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "创建成功", task)
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
	Name           string            `json:"name"`
	SourceDB       string            `json:"source_db"`
	SourceTable    string            `json:"source_table"`
	TargetDB       string            `json:"target_db"`
	TargetTable    string            `json:"target_table"`
	FieldMapping   map[string]string `json:"field_mapping"`
	SyncType       string            `json:"sync_type"`
	IncrementalKey string            `json:"incremental_key"`
	CronExpression string            `json:"cron_expression"`
	Status         *int              `json:"status"`
}

// UpdateTask 更新任务
func (h *SyncHandler) UpdateTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
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
	if req.IncrementalKey != "" {
		updates["incremental_key"] = req.IncrementalKey
	}
	if req.CronExpression != "" {
		updates["cron_expression"] = req.CronExpression
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if err := h.syncService.UpdateTask(uint(id), updates); err != nil {
		utils.InternalServerError(c, "更新任务失败: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "更新成功", nil)
}

// DeleteTask 删除任务
func (h *SyncHandler) DeleteTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	if err := h.syncService.DeleteTask(uint(id)); err != nil {
		utils.InternalServerError(c, "删除任务失败: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "删除成功", nil)
}

// ExecuteTask 手动执行任务
func (h *SyncHandler) ExecuteTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// 异步执行
	go h.syncService.ExecuteTask(uint(id))

	utils.SuccessWithMessage(c, "任务已开始执行", nil)
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
