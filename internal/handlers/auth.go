package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/redgreat/apiwong/internal/middleware"
	"github.com/redgreat/apiwong/internal/services"
	"github.com/redgreat/apiwong/internal/utils"
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

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 创建用户（默认角色为 user）
	user, err := h.authService.CreateUser(req.Username, req.Password, req.Email, "user")
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
	Email    string `json:"email"`
	Password string `json:"password,omitempty"`
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
	if req.Password != "" {
		updates["password"] = req.Password
	}

	if err := h.authService.UpdateUser(userID.(uint), updates); err != nil {
		utils.InternalServerError(c, "更新用户信息失败")
		return
	}

	utils.SuccessWithMessage(c, "更新成功", nil)
}
