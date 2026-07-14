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
	bytes, ok := jsonBytes(value)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, fm)
}

// Value 实现 driver.Valuer 接口
func (fm FieldMapping) Value() (driver.Value, error) {
	return json.Marshal(fm)
}

// StringList stores small JSON string arrays in metadata tables.
type StringList []string

func (sl *StringList) Scan(value interface{}) error {
	bytes, ok := jsonBytes(value)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, sl)
}

func (sl StringList) Value() (driver.Value, error) {
	return json.Marshal(sl)
}

func jsonBytes(value interface{}) ([]byte, bool) {
	switch typed := value.(type) {
	case []byte:
		return typed, true
	case string:
		return []byte(typed), true
	default:
		return nil, false
	}
}

// SyncTask 同步任务
type SyncTask struct {
	ID                   uint               `gorm:"primarykey" json:"id"`
	CreatedAt            time.Time          `json:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"`
	DeletedAt            gorm.DeletedAt     `gorm:"index" json:"-"`
	Name                 string             `gorm:"size:100;not null" json:"name"`                          // 任务名称
	SourceDB             string             `gorm:"size:50;not null" json:"source_db"`                      // 源数据库连接名
	SourceTable          string             `gorm:"size:100;not null" json:"source_table"`                  // 源表名
	TargetDB             string             `gorm:"size:50;not null" json:"target_db"`                      // 目标数据库连接名
	TargetTable          string             `gorm:"size:100;not null" json:"target_table"`                  // 目标表名
	FieldMapping         FieldMapping       `gorm:"type:json" json:"field_mapping"`                         // 字段映射 {"source_field": "target_field"}
	SyncType             string             `gorm:"size:20;not null" json:"sync_type"`                      // full, cdc, full_cdc
	IncrementalKey       string             `gorm:"size:100" json:"incremental_key"`                        // 增量同步的关键字段（如 updated_at, id）
	CronExpression       string             `gorm:"size:100" json:"cron_expression"`                        // Cron 表达式
	ScheduleType         string             `gorm:"size:20;not null;default:interval" json:"schedule_type"` // interval, cron, manual
	IntervalMinutes      int                `gorm:"not null;default:5" json:"interval_minutes"`
	Status               int                `gorm:"default:1" json:"status"`           // 1: 启用, 0: 禁用
	LastRunAt            *time.Time         `json:"last_run_at"`                       // 最后执行时间
	LastRunStatus        string             `gorm:"size:20" json:"last_run_status"`    // success, failed, running
	LastRunMessage       string             `gorm:"type:text" json:"last_run_message"` // 最后执行消息
	LastSuccessAt        *time.Time         `json:"last_success_at"`
	UserID               uint               `gorm:"not null" json:"user_id"`                 // 创建者
	AlertChannelID       *uint              `gorm:"index" json:"alert_channel_id,omitempty"` // 预警发送方
	AlertChannel         *AlertChannel      `gorm:"foreignKey:AlertChannelID" json:"alert_channel,omitempty"`
	AlertDelaySeconds    int                `gorm:"not null;default:0" json:"alert_delay_seconds"`     // 延迟预警阈值（秒）
	AlertStoppedMinutes  int                `gorm:"not null;default:0" json:"alert_stopped_minutes"`   // 停止预警阈值（分钟）
	AlertOnError         bool               `gorm:"not null;default:true" json:"alert_on_error"`       // 执行报错时预警
	AlertCooldownMinutes int                `gorm:"not null;default:30" json:"alert_cooldown_minutes"` // 预警重复间隔（分钟）
	ValidationStatus     string             `gorm:"size:20;not null;default:legacy" json:"validation_status"`
	RuntimeStatus        string             `gorm:"size:30;not null;default:stopped;index" json:"runtime_status"`
	SyncBatchSize        int                `gorm:"not null;default:0" json:"sync_batch_size"`
	SnapshotTableWorkers int                `gorm:"not null;default:0" json:"snapshot_table_workers"`
	SnapshotShardWorkers int                `gorm:"not null;default:0" json:"snapshot_shard_workers"`
	RowsProcessed        int64              `gorm:"not null;default:0" json:"rows_processed"`
	RowsPerSecond        float64            `gorm:"not null;default:0" json:"rows_per_second"`
	DelaySeconds         int64              `gorm:"not null;default:0" json:"delay_seconds"`
	PhaseStartedAt       *time.Time         `json:"phase_started_at"`
	RepairStatus         string             `gorm:"size:30;not null;default:idle;index" json:"repair_status"`
	TaskTables           []SyncTaskTable    `gorm:"foreignKey:TaskID" json:"task_tables,omitempty"`
	CDCCheckpoint        *SyncCDCCheckpoint `gorm:"foreignKey:TaskID;references:ID" json:"cdc_checkpoint,omitempty"`
}

// TableName 指定表名
func (SyncTask) TableName() string {
	return "sync_tasks"
}

// SyncTaskTable stores one source-to-target table mapping in a task.
type SyncTaskTable struct {
	ID                  uint         `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
	TaskID              uint         `gorm:"not null;index;uniqueIndex:uk_task_source_table" json:"task_id"`
	SourceTable         string       `gorm:"size:100;not null;uniqueIndex:uk_task_source_table" json:"source_table"`
	TargetTable         string       `gorm:"size:100;not null" json:"target_table"`
	IncrementalKey      string       `gorm:"size:100" json:"incremental_key"`
	FieldMapping        FieldMapping `gorm:"type:json" json:"field_mapping"`
	IgnoredFields       StringList   `gorm:"type:json" json:"ignored_fields"`
	TypeMismatchIgnores StringList   `gorm:"type:json" json:"type_mismatch_ignores"`
	Position            int          `gorm:"not null;default:0" json:"position"`
	SourcePrimaryKey    string       `gorm:"size:100" json:"source_primary_key"`
	TargetPrimaryKey    string       `gorm:"size:100" json:"target_primary_key"`
	SyncState           string       `gorm:"size:30;not null;default:pending;index" json:"sync_state"`
	SnapshotTotal       int64        `gorm:"not null;default:0" json:"snapshot_total"`
	SnapshotProcessed   int64        `gorm:"not null;default:0" json:"snapshot_processed"`
	ProgressPercent     float64      `gorm:"not null;default:0" json:"progress_percent"`
	OnboardingFile      string       `gorm:"size:255" json:"onboarding_file"`
	OnboardingPosition  uint32       `gorm:"not null;default:0" json:"onboarding_position"`
	ProgressMessage     string       `gorm:"type:text" json:"progress_message"`
	ActivatedAt         *time.Time   `json:"activated_at"`
}

func (SyncTaskTable) TableName() string { return "sync_task_tables" }

type SyncCheckpoint struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	TaskTableID      uint      `gorm:"not null;uniqueIndex" json:"task_table_id"`
	CursorValue      string    `gorm:"type:text" json:"cursor_value"`
	CursorPrimaryKey string    `gorm:"type:text" json:"cursor_primary_key"`
	Completed        bool      `gorm:"not null;default:false" json:"completed"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (SyncCheckpoint) TableName() string { return "sync_checkpoints" }

type SyncSnapshotShardCheckpoint struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	TaskTableID      uint      `gorm:"not null;uniqueIndex:uk_snapshot_shard" json:"task_table_id"`
	ShardIndex       int       `gorm:"not null;uniqueIndex:uk_snapshot_shard" json:"shard_index"`
	LowerBound       string    `gorm:"type:text" json:"lower_bound"`
	UpperBound       string    `gorm:"type:text" json:"upper_bound"`
	CursorPrimaryKey string    `gorm:"type:text" json:"cursor_primary_key"`
	ProcessedRows    int64     `gorm:"not null;default:0" json:"processed_rows"`
	Completed        bool      `gorm:"not null;default:false" json:"completed"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (SyncSnapshotShardCheckpoint) TableName() string { return "sync_snapshot_shard_checkpoints" }

// SyncCDCCheckpoint is the durable resume point for a task's MySQL binlog stream.
// The position is advanced only after the corresponding target transaction commits.
type SyncCDCCheckpoint struct {
	ID                uint       `gorm:"primarykey" json:"id"`
	TaskID            uint       `gorm:"not null;uniqueIndex" json:"task_id"`
	BinlogFile        string     `gorm:"size:255;not null" json:"binlog_file"`
	BinlogPosition    uint32     `gorm:"not null" json:"binlog_position"`
	SnapshotCompleted bool       `gorm:"not null;default:false" json:"snapshot_completed"`
	LastEventAt       *time.Time `json:"last_event_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (SyncCDCCheckpoint) TableName() string { return "sync_cdc_checkpoints" }

type SyncXAPreparedTransaction struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	TaskID         uint      `gorm:"not null;uniqueIndex:uk_task_xa" json:"task_id"`
	XIDKey         string    `gorm:"column:xid_key;size:255;not null;uniqueIndex:uk_task_xa" json:"xid_key"`
	BinlogFile     string    `gorm:"size:255;not null" json:"binlog_file"`
	BinlogPosition uint32    `gorm:"not null" json:"binlog_position"`
	OperationsJSON string    `gorm:"type:text;not null" json:"operations_json"`
}

func (SyncXAPreparedTransaction) TableName() string { return "sync_xa_prepared_transactions" }

// SyncLog 同步日志
type SyncLog struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	TaskID       uint      `gorm:"not null;index" json:"task_id"`           // 关联任务ID
	Status       string    `gorm:"size:20;not null" json:"status"`          // success, failed
	Message      string    `gorm:"type:text" json:"message"`                // 执行消息
	RowsAffected int64     `json:"rows_affected"`                           // 影响行数
	Duration     int64     `json:"duration"`                                // 执行时长（毫秒）
	ErrorDetail  string    `gorm:"type:text" json:"error_detail,omitempty"` // 错误详情
	TaskName     string    `gorm:"size:100;index" json:"task_name"`
	EventType    string    `gorm:"size:30;index" json:"event_type"`
	Phase        string    `gorm:"size:30;index" json:"phase"`
	Detail       string    `gorm:"type:text" json:"detail,omitempty"`
}

// TableName 指定表名
func (SyncLog) TableName() string {
	return "sync_logs"
}

// TaskAlertState records alert transitions and throttles repeated notifications.
type TaskAlertState struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	TaskID     uint       `gorm:"not null;uniqueIndex:uk_task_alert_type" json:"task_id"`
	AlertType  string     `gorm:"size:20;not null;uniqueIndex:uk_task_alert_type" json:"alert_type"`
	Active     bool       `gorm:"not null;default:false" json:"active"`
	AlertCount int        `gorm:"not null;default:0" json:"alert_count"` // 本轮已发送次数
	SilentAt   *time.Time `json:"silent_at"`                             // 进入静默的时刻
	LastSentAt *time.Time `json:"last_sent_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (TaskAlertState) TableName() string { return "task_alert_states" }

type ServerMonitorSetting struct {
	ID                 uint          `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
	Enabled            bool          `gorm:"not null;default:true" json:"enabled"`
	AlertChannelID     *uint         `gorm:"index" json:"alert_channel_id,omitempty"`
	AlertChannel       *AlertChannel `gorm:"foreignKey:AlertChannelID" json:"alert_channel,omitempty"`
	CPUThreshold       float64       `gorm:"not null;default:85" json:"cpu_threshold"`
	MemoryThreshold    float64       `gorm:"not null;default:85" json:"memory_threshold"`
	DiskThreshold      float64       `gorm:"not null;default:90" json:"disk_threshold"`
	GoroutineThreshold int           `gorm:"not null;default:20000" json:"goroutine_threshold"`
	LastAlertAt        *time.Time    `json:"last_alert_at"`
}

func (ServerMonitorSetting) TableName() string { return "server_monitor_settings" }

type SyncRepairJob struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	TaskID          uint       `gorm:"not null;index" json:"task_id"`
	JobType         string     `gorm:"size:20;not null;index" json:"job_type"` // compare, repair
	Status          string     `gorm:"size:20;not null;index" json:"status"`   // running, canceling, canceled, success, failed
	SourceJobID     uint       `gorm:"index" json:"source_job_id"`
	CutoffTime      *time.Time `json:"cutoff_time"`
	CutoffColumn    string     `gorm:"size:100" json:"cutoff_column"`
	TotalRows       int64      `gorm:"not null;default:0" json:"total_rows"`
	ProcessedRows   int64      `gorm:"not null;default:0" json:"processed_rows"`
	DiffRows        int64      `gorm:"not null;default:0" json:"diff_rows"`
	RepairedRows    int64      `gorm:"not null;default:0" json:"repaired_rows"`
	ProgressPercent float64    `gorm:"not null;default:0" json:"progress_percent"`
	Message         string     `gorm:"type:text" json:"message"`
	ErrorDetail     string     `gorm:"type:text" json:"error_detail,omitempty"`
	PreviousStatus  string     `gorm:"size:30" json:"previous_status"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
}

func (SyncRepairJob) TableName() string { return "sync_repair_jobs" }

type SyncRepairDiff struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	JobID       uint      `gorm:"not null;index;uniqueIndex:uk_repair_diff" json:"job_id"`
	TaskID      uint      `gorm:"not null;index" json:"task_id"`
	TaskTableID uint      `gorm:"not null;index;uniqueIndex:uk_repair_diff" json:"task_table_id"`
	SourceTable string    `gorm:"size:100;not null" json:"source_table"`
	TargetTable string    `gorm:"size:100;not null" json:"target_table"`
	SourcePK    string    `gorm:"size:255;not null;uniqueIndex:uk_repair_diff" json:"source_pk"`
	TargetPK    string    `gorm:"size:255" json:"target_pk"`
	DiffType    string    `gorm:"size:30;not null;index" json:"diff_type"` // missing_target, missing_source, mismatch
	SourceHash  string    `gorm:"size:64" json:"source_hash"`
	TargetHash  string    `gorm:"size:64" json:"target_hash"`
	Status      string    `gorm:"size:20;not null;default:pending;index" json:"status"` // pending, repaired, skipped, failed
	Message     string    `gorm:"type:text" json:"message"`
}

func (SyncRepairDiff) TableName() string { return "sync_repair_diffs" }
