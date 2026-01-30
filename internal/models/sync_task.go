package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// FieldMapping 字段映射
type FieldMapping map[string]string

// Scan 实现 sql.Scanner 接口
func (fm *FieldMapping) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, fm)
}

// Value 实现 driver.Valuer 接口
func (fm FieldMapping) Value() (driver.Value, error) {
	return json.Marshal(fm)
}

// SyncTask 同步任务
type SyncTask struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Name           string         `gorm:"size:100;not null" json:"name"`             // 任务名称
	SourceDB       string         `gorm:"size:50;not null" json:"source_db"`         // 源数据库连接名
	SourceTable    string         `gorm:"size:100;not null" json:"source_table"`     // 源表名
	TargetDB       string         `gorm:"size:50;not null" json:"target_db"`         // 目标数据库连接名
	TargetTable    string         `gorm:"size:100;not null" json:"target_table"`     // 目标表名
	FieldMapping   FieldMapping   `gorm:"type:json" json:"field_mapping"`            // 字段映射 {"source_field": "target_field"}
	SyncType       string         `gorm:"size:20;not null" json:"sync_type"`         // full: 全量, incremental: 增量
	IncrementalKey string         `gorm:"size:100" json:"incremental_key"`           // 增量同步的关键字段（如 updated_at, id）
	CronExpression string         `gorm:"size:100" json:"cron_expression"`           // Cron 表达式
	Status         int            `gorm:"default:1" json:"status"`                   // 1: 启用, 0: 禁用
	LastRunAt      *time.Time     `json:"last_run_at"`                               // 最后执行时间
	LastRunStatus  string         `gorm:"size:20" json:"last_run_status"`            // success, failed, running
	LastRunMessage string         `gorm:"type:text" json:"last_run_message"`         // 最后执行消息
	UserID         uint           `gorm:"not null" json:"user_id"`                   // 创建者
}

// TableName 指定表名
func (SyncTask) TableName() string {
	return "sync_tasks"
}

// SyncLog 同步日志
type SyncLog struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	TaskID       uint      `gorm:"not null;index" json:"task_id"`          // 关联任务ID
	Status       string    `gorm:"size:20;not null" json:"status"`         // success, failed
	Message      string    `gorm:"type:text" json:"message"`               // 执行消息
	RowsAffected int64     `json:"rows_affected"`                          // 影响行数
	Duration     int64     `json:"duration"`                               // 执行时长（毫秒）
	ErrorDetail  string    `gorm:"type:text" json:"error_detail,omitempty"` // 错误详情
}

// TableName 指定表名
func (SyncLog) TableName() string {
	return "sync_logs"
}
