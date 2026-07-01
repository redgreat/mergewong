package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redgreat/mergewong/internal/middleware"
	"github.com/redgreat/mergewong/internal/services"
	"github.com/redgreat/mergewong/internal/utils"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authService *services.AuthService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		authService: services.NewAuthService(),
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token    string `json:"token"`
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 验证用户
	user, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		utils.Unauthorized(c, err.Error())
		return
	}

	// 生成 JWT
	token, err := middleware.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		utils.InternalServerError(c, "生成令牌失败")
		return
	}

	// 返回响应
	utils.Success(c, LoginResponse{
		Token:    token,
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	})
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"required,email"`
}

// Register 用户注册（保留供内部调用；公开路由已关闭）
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	user, err := h.authService.CreateUser(req.Username, req.Password, req.Email, "viewer")
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "注册成功", gin.H{
		"user_id":  user.ID,
		"username": user.Username,
	})
}

// GetProfile 获取当前用户信息
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	user, err := h.authService.GetUserByID(userID.(uint))
	if err != nil {
		utils.InternalServerError(c, "获取用户信息失败")
		return
	}

	utils.Success(c, user)
}

// UpdateProfileRequest 更新用户信息请求
type UpdateProfileRequest struct {
	Email string `json:"email" binding:"omitempty,email"`
}

// UpdateProfile 更新当前用户信息
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.Email != "" {
		updates["email"] = req.Email
	}

	if err := h.authService.UpdateUser(userID.(uint), updates); err != nil {
		utils.InternalServerError(c, "更新用户信息失败")
		return
	}

	utils.SuccessWithMessage(c, "更新成功", nil)
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "密码至少需要 6 位")
		return
	}
	if err := h.authService.ChangePassword(userID.(uint), req.CurrentPassword, req.NewPassword); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessWithMessage(c, "密码修改成功，请重新登录", nil)
}

type AdminCreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"omitempty,email"`
	Role     string `json:"role" binding:"required,oneof=admin viewer"`
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	users, total, err := h.authService.ListUsers(page, pageSize)
	if err != nil {
		utils.InternalServerError(c, "获取用户列表失败")
		return
	}
	utils.Success(c, gin.H{"data": users, "total": total, "page": page, "page_size": pageSize})
}

func (h *AuthHandler) CreateUser(c *gin.Context) {
	var req AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	user, err := h.authService.CreateUser(req.Username, req.Password, req.Email, req.Role)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessWithMessage(c, "用户创建成功", user)
}

type AdminUpdateUserRequest struct {
	Email  *string `json:"email" binding:"omitempty,email"`
	Role   *string `json:"role" binding:"omitempty,oneof=admin viewer"`
	Status *int    `json:"status" binding:"omitempty,oneof=0 1"`
}

func (h *AuthHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "用户 ID 无效")
		return
	}
	user, err := h.authService.GetUserByID(uint(id))
	if err != nil {
		utils.Error(c, 404, "用户不存在")
		return
	}
	var req AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	nextRole, nextStatus := user.Role, user.Status
	updates := map[string]interface{}{}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Role != nil {
		nextRole = *req.Role
		updates["role"] = *req.Role
	}
	if req.Status != nil {
		nextStatus = *req.Status
		updates["status"] = *req.Status
	}
	if err := h.authService.EnsureAdminChangeSafe(user, nextRole, nextStatus); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.authService.UpdateUser(uint(id), updates); err != nil {
		utils.InternalServerError(c, "更新用户失败")
		return
	}
	utils.SuccessWithMessage(c, "用户已更新", nil)
}

func (h *AuthHandler) DeleteUser(c *gin.Context) {
	currentID, _ := c.Get("user_id")
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "用户 ID 无效")
		return
	}
	if currentID.(uint) == uint(id) {
		utils.BadRequest(c, "不能删除当前登录用户")
		return
	}
	user, err := h.authService.GetUserByID(uint(id))
	if err != nil {
		utils.Error(c, 404, "用户不存在")
		return
	}
	if err := h.authService.EnsureAdminChangeSafe(user, "viewer", 0); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.authService.DeleteUser(uint(id)); err != nil {
		utils.InternalServerError(c, "删除用户失败")
		return
	}
	utils.SuccessWithMessage(c, "用户已删除", nil)
}
