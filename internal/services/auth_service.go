package services

import (
	"errors"
	"fmt"

	"github.com/redgreat/apiwong/internal/database"
	"github.com/redgreat/apiwong/internal/models"
	"github.com/redgreat/apiwong/internal/utils"
	"gorm.io/gorm"
)

// AuthService 认证服务
type AuthService struct {
	db *gorm.DB
}

// NewAuthService 创建认证服务
func NewAuthService() *AuthService {
	db, _ := database.GetManager().GetConnection("system")
	return &AuthService{db: db}
}

// Login 用户登录
func (s *AuthService) Login(username, password string) (*models.User, error) {
	var user models.User

	// 查询用户
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}

	// 检查用户状态
	if user.Status == 0 {
		return nil, errors.New("用户已被禁用")
	}

	// 验证密码
	if !utils.CheckPasswordHash(password, user.Password) {
		return nil, errors.New("用户名或密码错误")
	}

	return &user, nil
}

// CreateUser 创建用户
func (s *AuthService) CreateUser(username, password, email, role string) (*models.User, error) {
	if role != "admin" && role != "viewer" {
		return nil, errors.New("角色只能是管理员或只读用户")
	}
	// 检查用户是否已存在
	var existingUser models.User
	if err := s.db.Where("username = ?", username).First(&existingUser).Error; err == nil {
		return nil, errors.New("用户名已存在")
	}

	// 加密密码
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// 创建用户
	user := models.User{
		Username: username,
		Password: hashedPassword,
		Email:    email,
		Role:     role,
		Status:   1,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

// GetUserByID 根据ID获取用户
func (s *AuthService) GetUserByID(id uint) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUser 更新用户信息
func (s *AuthService) UpdateUser(id uint, updates map[string]interface{}) error {
	if role, ok := updates["role"]; ok && role != "admin" && role != "viewer" {
		return errors.New("角色只能是管理员或只读用户")
	}
	// 如果更新密码，需要加密
	if password, ok := updates["password"]; ok {
		hashedPassword, err := utils.HashPassword(password.(string))
		if err != nil {
			return err
		}
		updates["password"] = hashedPassword
	}

	return s.db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
}

// ChangePassword 校验当前密码后修改用户密码。
func (s *AuthService) ChangePassword(id uint, currentPassword, newPassword string) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}
	if !utils.CheckPasswordHash(currentPassword, user.Password) {
		return errors.New("当前密码不正确")
	}
	if currentPassword == newPassword {
		return errors.New("新密码不能与当前密码相同")
	}
	return s.UpdateUser(id, map[string]interface{}{"password": newPassword})
}

// DeleteUser 删除用户
func (s *AuthService) DeleteUser(id uint) error {
	return s.db.Delete(&models.User{}, id).Error
}

// CountEnabledAdmins 返回启用的管理员数量。
func (s *AuthService) CountEnabledAdmins() (int64, error) {
	var count int64
	err := s.db.Model(&models.User{}).Where("role = ? AND status = ?", "admin", 1).Count(&count).Error
	return count, err
}

// EnsureAdminChangeSafe 防止系统失去最后一个可用管理员。
func (s *AuthService) EnsureAdminChangeSafe(user *models.User, nextRole string, nextStatus int) error {
	if user.Role != "admin" || user.Status != 1 || (nextRole == "admin" && nextStatus == 1) {
		return nil
	}
	count, err := s.CountEnabledAdmins()
	if err != nil {
		return err
	}
	if count <= 1 {
		return fmt.Errorf("不能禁用或降级最后一个管理员")
	}
	return nil
}

// ListUsers 列出所有用户
func (s *AuthService) ListUsers(page, pageSize int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	// 计算总数
	s.db.Model(&models.User{}).Count(&total)

	// 分页查询
	offset := (page - 1) * pageSize
	if err := s.db.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
