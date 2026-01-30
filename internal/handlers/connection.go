package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redgreat/apiwong/internal/models"
	"github.com/redgreat/apiwong/internal/services"
	"github.com/redgreat/apiwong/internal/utils"
)

type ConnectionHandler struct {
	connectionService *services.ConnectionService
}

func NewConnectionHandler() *ConnectionHandler {
	return &ConnectionHandler{
		connectionService: services.NewConnectionService(),
	}
}

type CreateConnectionRequest struct {
	Name     string `json:"name" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required"`
	Database string `json:"database" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Charset  string `json:"charset"`
	MaxIdle  int    `json:"max_idle"`
	MaxOpen  int    `json:"max_open"`
	Status   *int   `json:"status"`
}

func (h *ConnectionHandler) CreateConnection(c *gin.Context) {
	var req CreateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	status := 1
	if req.Status != nil {
		status = *req.Status
	}

	connection := &models.DatabaseConnection{
		Name:     req.Name,
		Type:     req.Type,
		Host:     req.Host,
		Port:     req.Port,
		Database: req.Database,
		Username: req.Username,
		Password: req.Password,
		Charset:  req.Charset,
		MaxIdle:  req.MaxIdle,
		MaxOpen:  req.MaxOpen,
		Status:   status,
		UserID:   userID.(uint),
	}

	if connection.Charset == "" {
		connection.Charset = "utf8mb4"
	}
	if connection.MaxIdle == 0 {
		connection.MaxIdle = 10
	}
	if connection.MaxOpen == 0 {
		connection.MaxOpen = 100
	}

	if connection.Status == 1 {
		if err := h.connectionService.TestConnection(connection); err != nil {
			utils.BadRequest(c, "连接测试失败: "+err.Error())
			return
		}
	}

	if err := h.connectionService.CreateConnection(connection); err != nil {
		utils.InternalServerError(c, "创建连接失败: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "创建成功", connection)
}

func (h *ConnectionHandler) ListConnections(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	connections, total, err := h.connectionService.ListConnections(page, pageSize)
	if err != nil {
		utils.InternalServerError(c, "获取连接列表失败: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"data":      connections,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *ConnectionHandler) GetConnection(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	connection, err := h.connectionService.GetConnection(uint(id))
	if err != nil {
		utils.Error(c, 404, "连接不存在")
		return
	}

	utils.Success(c, connection)
}

type UpdateConnectionRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     *int   `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	Charset  string `json:"charset"`
	MaxIdle  *int   `json:"max_idle"`
	MaxOpen  *int   `json:"max_open"`
	Status   *int   `json:"status"`
}

func (h *ConnectionHandler) UpdateConnection(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req UpdateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	existing, err := h.connectionService.GetConnection(uint(id))
	if err != nil {
		utils.Error(c, 404, "连接不存在")
		return
	}

	updated := *existing
	if req.Name != "" {
		updated.Name = req.Name
	}
	if req.Type != "" {
		updated.Type = req.Type
	}
	if req.Host != "" {
		updated.Host = req.Host
	}
	if req.Port != nil {
		updated.Port = *req.Port
	}
	if req.Database != "" {
		updated.Database = req.Database
	}
	if req.Username != "" {
		updated.Username = req.Username
	}
	if req.Password != "" {
		updated.Password = req.Password
	}
	if req.Charset != "" {
		updated.Charset = req.Charset
	}
	if req.MaxIdle != nil {
		updated.MaxIdle = *req.MaxIdle
	}
	if req.MaxOpen != nil {
		updated.MaxOpen = *req.MaxOpen
	}
	if req.Status != nil {
		updated.Status = *req.Status
	}

	if updated.Charset == "" {
		updated.Charset = "utf8mb4"
	}
	if updated.MaxIdle == 0 {
		updated.MaxIdle = 10
	}
	if updated.MaxOpen == 0 {
		updated.MaxOpen = 100
	}

	if updated.Status == 1 {
		if err := h.connectionService.TestConnection(&updated); err != nil {
			utils.BadRequest(c, "连接测试失败: "+err.Error())
			return
		}
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = updated.Name
	}
	if req.Type != "" {
		updates["type"] = updated.Type
	}
	if req.Host != "" {
		updates["host"] = updated.Host
	}
	if req.Port != nil {
		updates["port"] = updated.Port
	}
	if req.Database != "" {
		updates["database"] = updated.Database
	}
	if req.Username != "" {
		updates["username"] = updated.Username
	}
	if req.Password != "" {
		updates["password"] = updated.Password
	}
	if req.Charset != "" {
		updates["charset"] = updated.Charset
	}
	if req.MaxIdle != nil {
		updates["max_idle"] = updated.MaxIdle
	}
	if req.MaxOpen != nil {
		updates["max_open"] = updated.MaxOpen
	}
	if req.Status != nil {
		updates["status"] = updated.Status
	}

	if err := h.connectionService.UpdateConnection(uint(id), updates); err != nil {
		utils.InternalServerError(c, "更新连接失败: "+err.Error())
		return
	}

	if err := h.connectionService.RefreshManager(existing, &updated); err != nil {
		utils.InternalServerError(c, "更新连接失败: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "更新成功", nil)
}

func (h *ConnectionHandler) DeleteConnection(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	if err := h.connectionService.DeleteConnection(uint(id)); err != nil {
		utils.InternalServerError(c, "删除连接失败: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "删除成功", nil)
}

func (h *ConnectionHandler) TestConnection(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	connection, err := h.connectionService.GetConnection(uint(id))
	if err != nil {
		utils.Error(c, 404, "连接不存在")
		return
	}

	if err := h.connectionService.TestConnection(connection); err != nil {
		utils.BadRequest(c, "连接测试失败: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "连接正常", nil)
}
