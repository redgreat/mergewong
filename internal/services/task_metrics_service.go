package services

import (
	"encoding/json"
	"time"

	"github.com/redgreat/mergewong/internal/models"
)

const (
	taskMetricEventType = "cdc_metrics"
	taskMetricRetention = 30 * 24 * time.Hour
	taskMetricInterval  = time.Minute
)

type TaskMetricPoint struct {
	Time          time.Time `json:"time"`
	DelaySeconds  int64     `json:"delay_seconds"`
	RowsPerSecond float64   `json:"rows_per_second"`
	InsertRows    int64     `json:"insert_rows"`
	UpdateRows    int64     `json:"update_rows"`
	DeleteRows    int64     `json:"delete_rows"`
	ReadRows      int64     `json:"read_rows"`
	TotalRows     int64     `json:"total_rows"`
}

type cdcMetricLogDetail struct {
	DelaySeconds  int64   `json:"delay_seconds"`
	RowsPerSecond float64 `json:"rows_per_second"`
	InsertRows    int64   `json:"insert_rows"`
	UpdateRows    int64   `json:"update_rows"`
	DeleteRows    int64   `json:"delete_rows"`
	ReadRows      int64   `json:"read_rows"`
	TotalRows     int64   `json:"total_rows"`
}

func (s *SyncService) RecordCDCMetricSnapshot(task *models.SyncTask, now time.Time, delay int64, speed float64, sessionRows int64, lastLog *time.Time, lastRows *int64, lastOps *cdcOperationMetrics, currentOps cdcOperationMetrics) error {
	if task == nil || lastLog == nil || lastRows == nil || lastOps == nil {
		return nil
	}
	deltaRows := sessionRows - *lastRows
	deltaOps := cdcOperationMetrics{
		Insert: currentOps.Insert - lastOps.Insert,
		Update: currentOps.Update - lastOps.Update,
		Delete: currentOps.Delete - lastOps.Delete,
	}
	if !lastLog.IsZero() && now.Sub(*lastLog) < taskMetricInterval && deltaRows <= 0 {
		return nil
	}
	if deltaRows <= 0 && delay <= 0 {
		return nil
	}
	insertRows := maxInt64(deltaOps.Insert, 0)
	updateRows := maxInt64(deltaOps.Update, 0)
	deleteRows := maxInt64(deltaOps.Delete, 0)
	totalRows := maxInt64(deltaRows, 0)
	readRows := totalRows - insertRows - updateRows - deleteRows
	if readRows < 0 {
		readRows = 0
	}
	detail := cdcMetricLogDetail{
		DelaySeconds:  delay,
		RowsPerSecond: speed,
		InsertRows:    insertRows,
		UpdateRows:    updateRows,
		DeleteRows:    deleteRows,
		ReadRows:      readRows,
		TotalRows:     totalRows,
	}
	bytes, _ := json.Marshal(detail)
	log := models.SyncLog{
		CreatedAt:    now,
		TaskID:       task.ID,
		TaskName:     task.Name,
		EventType:    taskMetricEventType,
		Phase:        "cdc",
		Status:       "success",
		Message:      "同步指标快照",
		Detail:       string(bytes),
		RowsAffected: deltaRows,
		Duration:     delay * 1000,
	}
	if err := s.systemDB.Create(&log).Error; err != nil {
		return err
	}
	*lastLog = now
	*lastRows = sessionRows
	*lastOps = currentOps
	cutoff := now.Add(-taskMetricRetention)
	return s.systemDB.Where("event_type = ? AND created_at < ?", taskMetricEventType, cutoff).Delete(&models.SyncLog{}).Error
}

func (s *SyncService) GetTaskMetricHistory(taskID uint, from, to time.Time) ([]TaskMetricPoint, error) {
	if to.IsZero() {
		to = time.Now()
	}
	if from.IsZero() {
		from = to.Add(-24 * time.Hour)
	}
	retentionStart := time.Now().Add(-taskMetricRetention)
	if from.Before(retentionStart) {
		from = retentionStart
	}
	if to.Before(from) {
		to = from.Add(time.Hour)
	}
	var logs []models.SyncLog
	err := s.systemDB.Where("task_id = ? AND created_at BETWEEN ? AND ? AND event_type IN ?", taskID, from, to, []string{taskMetricEventType, "snapshot_completed", "table_snapshot_completed"}).
		Order("created_at ASC").
		Find(&logs).Error
	if err != nil {
		return nil, err
	}
	bucketSize := time.Minute
	if to.Sub(from) > 48*time.Hour {
		bucketSize = time.Hour
	}
	pointsByTime := map[time.Time]*TaskMetricPoint{}
	order := []time.Time{}
	for _, log := range logs {
		bucket := log.CreatedAt.Truncate(bucketSize)
		point := pointsByTime[bucket]
		if point == nil {
			point = &TaskMetricPoint{Time: bucket}
			pointsByTime[bucket] = point
			order = append(order, bucket)
		}
		if log.EventType == taskMetricEventType {
			var detail cdcMetricLogDetail
			_ = json.Unmarshal([]byte(log.Detail), &detail)
			if detail.DelaySeconds > point.DelaySeconds {
				point.DelaySeconds = detail.DelaySeconds
			}
			point.RowsPerSecond = detail.RowsPerSecond
			point.ReadRows += detail.ReadRows
			point.InsertRows += detail.InsertRows
			point.UpdateRows += detail.UpdateRows
			point.DeleteRows += detail.DeleteRows
			point.TotalRows += detail.TotalRows
			continue
		}
		point.ReadRows += log.RowsAffected
		point.TotalRows += log.RowsAffected
	}
	points := make([]TaskMetricPoint, 0, len(order))
	for _, key := range order {
		points = append(points, *pointsByTime[key])
	}
	return points, nil
}

func maxInt64(value, fallback int64) int64 {
	if value < fallback {
		return fallback
	}
	return value
}
