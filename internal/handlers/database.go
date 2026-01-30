package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redgreat/apiwong/internal/services"
	"github.com/redgreat/apiwong/internal/utils"
)

// DatabaseHandler 数据库处理器
type DatabaseHandler struct {
	dbService *services.DBService
}

// NewDatabaseHandler 创建数据库处理器
func NewDatabaseHandler() *DatabaseHandler {
	return &DatabaseHandler{
		dbService: services.NewDBService(),
	}
}

// QueryRequest 查询请求
type QueryRequest struct {
	SQL      string        `json:"sql" binding:"required"`
	Params   []interface{} `json:"params"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// Query 执行查询
func (h *DatabaseHandler) Query(c *gin.Context) {
	dbName := c.Param("name")
	
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 设置默认分页
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	results, total, err := h.dbService.QueryData(dbName, req.SQL, req.Params, req.Page, req.PageSize)
	if err != nil {
		utils.InternalServerError(c, "查询失败: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"data":      results,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	})
}

// ExecRequest 执行请求
type ExecRequest struct {
	SQL    string        `json:"sql" binding:"required"`
	Params []interface{} `json:"params"`
}

// Exec 执行 SQL
func (h *DatabaseHandler) Exec(c *gin.Context) {
	dbName := c.Param("name")
	
	var req ExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	rowsAffected, err := h.dbService.ExecSQL(dbName, req.SQL, req.Params)
	if err != nil {
		utils.InternalServerError(c, "执行失败: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"rows_affected": rowsAffected,
	})
}

// ListTables 列出所有表
func (h *DatabaseHandler) ListTables(c *gin.Context) {
	dbName := c.Param("name")
	
	tables, err := h.dbService.ListTables(dbName)
	if err != nil {
		utils.InternalServerError(c, "获取表列表失败: "+err.Error())
		return
	}

	utils.Success(c, tables)
}

// GetTableSchema 获取表结构
func (h *DatabaseHandler) GetTableSchema(c *gin.Context) {
	dbName := c.Param("name")
	tableName := c.Param("table")
	
	schema, err := h.dbService.GetTableSchema(dbName, tableName)
	if err != nil {
		utils.InternalServerError(c, "获取表结构失败: "+err.Error())
		return
	}

	utils.Success(c, schema)
}

// InsertDataRequest 插入数据请求
type InsertDataRequest struct {
	Data map[string]interface{} `json:"data" binding:"required"`
}

// InsertData 插入数据
func (h *DatabaseHandler) InsertData(c *gin.Context) {
	dbName := c.Param("name")
	tableName := c.Param("table")
	
	var req InsertDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.dbService.InsertData(dbName, tableName, req.Data); err != nil {
		utils.InternalServerError(c, "插入数据失败: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "插入成功", nil)
}

// UpdateDataRequest 更新数据请求
type UpdateDataRequest struct {
	Data map[string]interface{} `json:"data" binding:"required"`
}

// UpdateData 更新数据
func (h *DatabaseHandler) UpdateData(c *gin.Context) {
	dbName := c.Param("name")
	tableName := c.Param("table")
	id := c.Param("id")
	
	var req UpdateDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 尝试将 id 转换为整数，如果失败则作为字符串处理
	var idValue interface{}
	if idInt, err := strconv.Atoi(id); err == nil {
		idValue = idInt
	} else {
		idValue = id
	}

	if err := h.dbService.UpdateData(dbName, tableName, "id", idValue, req.Data); err != nil {
		utils.InternalServerError(c, "更新数据失败: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "更新成功", nil)
}

// DeleteData 删除数据
func (h *DatabaseHandler) DeleteData(c *gin.Context) {
	dbName := c.Param("name")
	tableName := c.Param("table")
	id := c.Param("id")
	
	// 尝试将 id 转换为整数，如果失败则作为字符串处理
	var idValue interface{}
	if idInt, err := strconv.Atoi(id); err == nil {
		idValue = idInt
	} else {
		idValue = id
	}

	if err := h.dbService.DeleteData(dbName, tableName, "id", idValue); err != nil {
		utils.InternalServerError(c, "删除数据失败: "+err.Error())
		return
	}

	utils.SuccessWithMessage(c, "删除成功", nil)
}
