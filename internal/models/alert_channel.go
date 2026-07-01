package models

import (
	"time"

	"gorm.io/gorm"
)

// AlertChannel 企业微信机器人预警发送方。
type AlertChannel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"size:100;not null;uniqueIndex" json:"name"`
	RobotID   string         `gorm:"size:500;not null" json:"-"`
	Status    int            `gorm:"default:1;not null" json:"status"`
}

func (AlertChannel) TableName() string { return "alert_channels" }

// AlertChannelView 是管理端可见的发送方信息，不暴露机器人 ID。
type AlertChannelView struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	RobotIDMask string    `json:"robot_id_mask"`
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
