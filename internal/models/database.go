package models

import (
	"time"

	"gorm.io/gorm"
)

// DatabaseConnection 数据库连接配置
type DatabaseConnection struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"uniqueIndex;size:50;not null" json:"name"` // 连接名称
	Type      string         `gorm:"size:20;not null" json:"type"`             // mysql, postgres, sqlserver, oracle
	Host      string         `gorm:"size:100;not null" json:"host"`
	Port      int            `gorm:"not null" json:"port"`
	Database  string         `gorm:"size:100;not null" json:"database"`
	Username  string         `gorm:"size:100;not null" json:"username"`
	Password  string         `gorm:"size:255;not null" json:"-"` // 不返回给前端
	Charset   string         `gorm:"size:20;default:'utf8mb4'" json:"charset"`
	MaxIdle   int            `gorm:"default:10" json:"max_idle"`
	MaxOpen   int            `gorm:"default:100" json:"max_open"`
	Status    int            `gorm:"default:1" json:"status"` // 1: 启用, 0: 禁用
	UserID    uint           `gorm:"not null" json:"user_id"` // 创建者
}

// TableName 指定表名
func (DatabaseConnection) TableName() string {
	return "database_connections"
}
