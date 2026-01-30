package models

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Username  string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password  string         `gorm:"size:255;not null" json:"-"` // 密码哈希，不返回给前端
	Email     string         `gorm:"size:100" json:"email"`
	Role      string         `gorm:"size:20;default:'user'" json:"role"` // admin, user
	Status    int            `gorm:"default:1" json:"status"`            // 1: 启用, 0: 禁用
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
