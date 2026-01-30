package services

import (
	"errors"

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

// DeleteUser 删除用户
func (s *AuthService) DeleteUser(id uint) error {
	return s.db.Delete(&models.User{}, id).Error
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
