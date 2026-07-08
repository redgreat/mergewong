package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redgreat/mergewong/internal/models"
	"github.com/redgreat/mergewong/internal/services"
	"github.com/redgreat/mergewong/internal/utils"
)

type ServerMonitorHandler struct {
	service *services.ServerMonitorService
}

func NewServerMonitorHandler() *ServerMonitorHandler {
	return &ServerMonitorHandler{service: services.NewServerMonitorService()}
}

type serverMonitorSettingRequest struct {
	Enabled            bool    `json:"enabled"`
	AlertChannelID     *uint   `json:"alert_channel_id"`
	CPUThreshold       float64 `json:"cpu_threshold"`
	MemoryThreshold    float64 `json:"memory_threshold"`
	DiskThreshold      float64 `json:"disk_threshold"`
	GoroutineThreshold int     `json:"goroutine_threshold"`
}

func (h *ServerMonitorHandler) Metrics(c *gin.Context) {
	metrics, err := h.service.Metrics()
	if err != nil {
		utils.InternalServerError(c, "获取服务器指标失败: "+err.Error())
		return
	}
	utils.Success(c, metrics)
}

func (h *ServerMonitorHandler) GetSetting(c *gin.Context) {
	setting, err := h.service.GetSetting()
	if err != nil {
		utils.InternalServerError(c, "获取服务器预警配置失败: "+err.Error())
		return
	}
	utils.Success(c, setting)
}

func (h *ServerMonitorHandler) SaveSetting(c *gin.Context) {
	var req serverMonitorSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	alertChannelID := req.AlertChannelID
	if alertChannelID != nil && *alertChannelID == 0 {
		alertChannelID = nil
	}
	if alertChannelID != nil {
		if err := services.NewSyncService().ValidateAlertChannelID(*alertChannelID); err != nil {
			utils.BadRequest(c, strings.ReplaceAll(err.Error(), "预警发送方", "服务器预警发送方"))
			return
		}
	}
	setting := &models.ServerMonitorSetting{
		ID:                 1,
		Enabled:            req.Enabled,
		AlertChannelID:     alertChannelID,
		CPUThreshold:       req.CPUThreshold,
		MemoryThreshold:    req.MemoryThreshold,
		DiskThreshold:      req.DiskThreshold,
		GoroutineThreshold: req.GoroutineThreshold,
	}
	if err := h.service.SaveSetting(setting); err != nil {
		utils.InternalServerError(c, "保存服务器预警配置失败: "+err.Error())
		return
	}
	utils.SuccessWithMessage(c, "保存成功", setting)
}
