package handlers

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redgreat/apiwong/internal/models"
	"github.com/redgreat/apiwong/internal/services"
	"github.com/redgreat/apiwong/internal/utils"
)

type AlertHandler struct{ service *services.AlertService }

func NewAlertHandler() *AlertHandler { return &AlertHandler{service: services.NewAlertService()} }

type alertRequest struct {
	Name    string `json:"name" binding:"required,max=100"`
	RobotID string `json:"robot_id"`
	Status  *int   `json:"status"`
}

func (h *AlertHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	enabledOnly := c.Query("enabled_only") == "true"
	channels, total, err := h.service.List(page, pageSize, enabledOnly)
	if err != nil {
		utils.InternalServerError(c, "获取预警发送方失败: "+err.Error())
		return
	}
	utils.Success(c, gin.H{"data": channels, "total": total, "page": page, "page_size": pageSize})
}

func (h *AlertHandler) Create(c *gin.Context) {
	var req alertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	if strings.TrimSpace(req.RobotID) == "" {
		utils.BadRequest(c, "企业微信机器人 ID 不能为空")
		return
	}
	robotID, err := services.NormalizeWecomRobotID(req.RobotID)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	status := 1
	if req.Status != nil {
		status = *req.Status
	}
	channel := &models.AlertChannel{Name: strings.TrimSpace(req.Name), RobotID: robotID, Status: status}
	if err := h.service.Create(channel); err != nil {
		utils.InternalServerError(c, "创建预警发送方失败: "+err.Error())
		return
	}
	utils.SuccessWithMessage(c, "创建成功", gin.H{"id": channel.ID})
}

func (h *AlertHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req alertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	updates := map[string]interface{}{"name": strings.TrimSpace(req.Name)}
	if strings.TrimSpace(req.RobotID) != "" {
		robotID, err := services.NormalizeWecomRobotID(req.RobotID)
		if err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		updates["robot_id"] = robotID
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if err := h.service.Update(uint(id), updates); err != nil {
		utils.InternalServerError(c, "更新预警发送方失败: "+err.Error())
		return
	}
	utils.SuccessWithMessage(c, "更新成功", nil)
}

func (h *AlertHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.Delete(uint(id)); err != nil {
		utils.BadRequest(c, "删除预警发送方失败: "+err.Error())
		return
	}
	utils.SuccessWithMessage(c, "删除成功", nil)
}

func (h *AlertHandler) Test(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.service.Test(c.Request.Context(), uint(id)); err != nil {
		utils.BadRequest(c, "测试发送失败: "+err.Error())
		return
	}
	utils.SuccessWithMessage(c, "测试消息已发送", nil)
}
