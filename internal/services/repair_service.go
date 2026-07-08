package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redgreat/mergewong/internal/database"
	"github.com/redgreat/mergewong/internal/models"
	"gorm.io/gorm"
)

const repairBatchSize = 5000

type RepairService struct {
	systemDB *gorm.DB
}

type RepairCompareRequest struct {
	CutoffTime   *time.Time `json:"cutoff_time"`
	CutoffColumn string     `json:"cutoff_column"`
}

var repairCancels sync.Map

func NewRepairService() *RepairService {
	db, _ := database.GetManager().GetConnection("system")
	return &RepairService{systemDB: db}
}

func (s *RepairService) ListJobs(taskID uint) ([]models.SyncRepairJob, error) {
	var jobs []models.SyncRepairJob
	err := s.systemDB.Where("task_id = ?", taskID).Order("id DESC").Limit(10).Find(&jobs).Error
	return jobs, err
}

func (s *RepairService) ListDiffs(jobID uint, page, pageSize int) ([]models.SyncRepairDiff, int64, error) {
	var total int64
	query := s.systemDB.Model(&models.SyncRepairDiff{}).Where("job_id = ?", jobID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var diffs []models.SyncRepairDiff
	err := query.Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&diffs).Error
	return diffs, total, err
}

func (s *RepairService) StartCompare(taskID uint, req RepairCompareRequest) (*models.SyncRepairJob, error) {
	task, err := NewSyncService().GetTask(taskID)
	if err != nil {
		return nil, err
	}
	req.CutoffColumn = strings.TrimSpace(req.CutoffColumn)
	if req.CutoffColumn != "" && !taskIdentifierPattern.MatchString(req.CutoffColumn) {
		return nil, fmt.Errorf("截止字段名不合法")
	}
	if task.ValidationStatus != "passed" {
		return nil, fmt.Errorf("任务预检查尚未通过")
	}
	if err := s.ensureNoRunningJob(taskID); err != nil {
		return nil, err
	}
	now := time.Now()
	job := &models.SyncRepairJob{TaskID: taskID, JobType: "compare", Status: "running", CutoffTime: req.CutoffTime, CutoffColumn: req.CutoffColumn, Message: "正在对比", StartedAt: &now}
	if err := s.systemDB.Create(job).Error; err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	repairCancels.Store(job.ID, cancel)
	_ = NewSyncService().UpdateTask(taskID, map[string]interface{}{"repair_status": "comparing"})
	go s.runCompare(ctx, job.ID)
	return job, nil
}

func (s *RepairService) StartRepair(taskID, compareJobID uint) (*models.SyncRepairJob, error) {
	task, err := NewSyncService().GetTask(taskID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureNoRunningJob(taskID); err != nil {
		return nil, err
	}
	var compare models.SyncRepairJob
	if err := s.systemDB.Where("id = ? AND task_id = ? AND job_type = ?", compareJobID, taskID, "compare").First(&compare).Error; err != nil {
		return nil, fmt.Errorf("对比任务不存在")
	}
	now := time.Now()
	job := &models.SyncRepairJob{TaskID: taskID, JobType: "repair", Status: "running", SourceJobID: compare.ID, CutoffTime: compare.CutoffTime, CutoffColumn: compare.CutoffColumn, Message: "正在补数", PreviousStatus: task.RuntimeStatus, StartedAt: &now}
	if err := s.systemDB.Create(job).Error; err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	repairCancels.Store(job.ID, cancel)
	go s.runRepair(ctx, job.ID)
	return job, nil
}

func (s *RepairService) CancelJob(jobID uint) error {
	var job models.SyncRepairJob
	if err := s.systemDB.First(&job, jobID).Error; err != nil {
		return err
	}
	if job.Status != "running" {
		return nil
	}
	_ = s.systemDB.Model(&job).Update("status", "canceling").Error
	if cancel, ok := repairCancels.Load(jobID); ok {
		cancel.(context.CancelFunc)()
	}
	return nil
}

func (s *RepairService) ensureNoRunningJob(taskID uint) error {
	var count int64
	if err := s.systemDB.Model(&models.SyncRepairJob{}).Where("task_id = ? AND status IN ?", taskID, []string{"running", "canceling"}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("已有数据修复任务正在执行")
	}
	return nil
}

func (s *RepairService) runCompare(ctx context.Context, jobID uint) {
	defer repairCancels.Delete(jobID)
	var job models.SyncRepairJob
	if err := s.systemDB.First(&job, jobID).Error; err != nil {
		return
	}
	err := s.compareJob(ctx, &job)
	s.finishJob(ctx, &job, err, "对比完成")
}

func (s *RepairService) compareJob(ctx context.Context, job *models.SyncRepairJob) error {
	task, err := NewSyncService().GetTask(job.TaskID)
	if err != nil {
		return err
	}
	sourceDB, err := database.GetManager().GetConnection(task.SourceDB)
	if err != nil {
		return err
	}
	targetDB, err := database.GetManager().GetConnection(task.TargetDB)
	if err != nil {
		return err
	}
	_ = s.systemDB.Where("job_id = ?", job.ID).Delete(&models.SyncRepairDiff{}).Error
	for i := range task.TaskTables {
		if err := s.compareTable(ctx, job, task, &task.TaskTables[i], sourceDB, targetDB); err != nil {
			return err
		}
	}
	return nil
}

func (s *RepairService) compareTable(ctx context.Context, job *models.SyncRepairJob, task *models.SyncTask, mapping *models.SyncTaskTable, sourceDB, targetDB *gorm.DB) error {
	sourceColumns, err := selectableSourceColumns(task, mapping, sourceDB)
	if err != nil {
		return err
	}
	pairs, err := syncColumnPairs(targetDB, mapping, sourceColumns)
	if err != nil {
		return err
	}
	lastPK := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := readRepairRows(sourceDB, mapping.SourceTable, mapping.SourcePrimaryKey, sourcePairColumns(pairs), lastPK, job.CutoffColumn, job.CutoffTime)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			lastPK = valueString(row[mapping.SourcePrimaryKey])
			targetRow, err := readSingleTargetRow(targetDB, mapping.TargetTable, mapping.TargetPrimaryKey, targetPairColumns(pairs), lastPK)
			if err != nil {
				return err
			}
			sourceHash := hashRepairRow(row, pairs, true)
			if targetRow == nil {
				s.recordDiff(job, mapping, lastPK, lastPK, "missing_target", sourceHash, "", "目标缺少数据")
			} else {
				targetHash := hashRepairRow(targetRow, pairs, false)
				if sourceHash != targetHash {
					s.recordDiff(job, mapping, lastPK, lastPK, "mismatch", sourceHash, targetHash, "字段值不一致")
				}
			}
			s.bumpJobProgress(job.ID, 1, 0, 0)
		}
	}
	return s.compareTargetExtras(ctx, job, mapping, sourceDB, targetDB, pairs)
}

func (s *RepairService) compareTargetExtras(ctx context.Context, job *models.SyncRepairJob, mapping *models.SyncTaskTable, sourceDB, targetDB *gorm.DB, pairs []syncColumnPair) error {
	lastPK := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := readRepairRows(targetDB, mapping.TargetTable, mapping.TargetPrimaryKey, targetPairColumns(pairs), lastPK, "", nil)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			lastPK = valueString(row[mapping.TargetPrimaryKey])
			exists, err := sourcePKExists(sourceDB, mapping.SourceTable, mapping.SourcePrimaryKey, lastPK, job.CutoffColumn, job.CutoffTime)
			if err != nil {
				return err
			}
			if !exists {
				s.recordDiff(job, mapping, lastPK, lastPK, "missing_source", "", hashRepairRow(row, pairs, false), "源端缺少数据")
			}
		}
	}
}

func (s *RepairService) runRepair(ctx context.Context, jobID uint) {
	defer repairCancels.Delete(jobID)
	var job models.SyncRepairJob
	if err := s.systemDB.First(&job, jobID).Error; err != nil {
		return
	}
	err := s.repairJob(ctx, &job)
	s.finishJob(ctx, &job, err, "补数完成")
}

func (s *RepairService) repairJob(ctx context.Context, job *models.SyncRepairJob) error {
	syncSvc := NewSyncService()
	task, err := syncSvc.GetTask(job.TaskID)
	if err != nil {
		return err
	}
	wasRunning := task.RuntimeStatus == "initializing" || task.RuntimeStatus == "catching_up" || task.RuntimeStatus == "cdc_running"
	if wasRunning {
		if err := syncSvc.PauseTask(task.ID); err != nil {
			return err
		}
	}
	_ = syncSvc.UpdateTask(task.ID, map[string]interface{}{"repair_status": "repairing", "last_run_message": "正在补数，预警已暂停"})
	defer func() {
		if wasRunning {
			_ = syncSvc.ResumeTask(task.ID)
		} else {
			_ = syncSvc.UpdateTask(task.ID, map[string]interface{}{"repair_status": "idle"})
		}
	}()

	sourceDB, err := database.GetManager().GetConnection(task.SourceDB)
	if err != nil {
		return err
	}
	targetDB, err := database.GetManager().GetConnection(task.TargetDB)
	if err != nil {
		return err
	}
	var diffs []models.SyncRepairDiff
	diffJobID := job.SourceJobID
	if diffJobID == 0 {
		diffJobID = job.ID
	}
	if err := s.systemDB.Where("job_id = ? AND status = ? AND diff_type IN ?", diffJobID, "pending", []string{"missing_target", "mismatch"}).Order("id ASC").Find(&diffs).Error; err != nil {
		return err
	}
	tableByID := map[uint]*models.SyncTaskTable{}
	for i := range task.TaskTables {
		tableByID[task.TaskTables[i].ID] = &task.TaskTables[i]
	}
	diffsByTable := map[uint][]models.SyncRepairDiff{}
	for _, diff := range diffs {
		diffsByTable[diff.TaskTableID] = append(diffsByTable[diff.TaskTableID], diff)
	}
	for tableID, tableDiffs := range diffsByTable {
		if err := ctx.Err(); err != nil {
			return err
		}
		mapping := tableByID[tableID]
		if mapping == nil {
			continue
		}
		sourceColumns, err := selectableSourceColumns(task, mapping, sourceDB)
		if err != nil {
			return err
		}
		for start := 0; start < len(tableDiffs); start += repairBatchSize {
			end := start + repairBatchSize
			if end > len(tableDiffs) {
				end = len(tableDiffs)
			}
			chunk := tableDiffs[start:end]
			rowsByPK, err := readSourceRowsByPKs(sourceDB, mapping.SourceTable, mapping.SourcePrimaryKey, sourceColumns, repairDiffPKs(chunk))
			if err != nil {
				return err
			}
			writeRows := make([]map[string]interface{}, 0, len(chunk))
			repairedIDs := make([]uint, 0, len(chunk))
			skippedIDs := make([]uint, 0)
			for _, diff := range chunk {
				row := rowsByPK[diff.SourcePK]
				if row == nil {
					skippedIDs = append(skippedIDs, diff.ID)
					continue
				}
				writeRows = append(writeRows, row)
				repairedIDs = append(repairedIDs, diff.ID)
			}
			if len(writeRows) > 0 {
				if err := writeMySQLBatch(targetDB, mapping, sourceColumns, writeRows); err != nil {
					_ = s.systemDB.Model(&models.SyncRepairDiff{}).Where("id IN ?", repairedIDs).Updates(map[string]interface{}{"status": "failed", "message": err.Error()}).Error
					return err
				}
				_ = s.systemDB.Model(&models.SyncRepairDiff{}).Where("id IN ?", repairedIDs).Updates(map[string]interface{}{"status": "repaired", "message": "已补数"}).Error
			}
			if len(skippedIDs) > 0 {
				_ = s.systemDB.Model(&models.SyncRepairDiff{}).Where("id IN ?", skippedIDs).Updates(map[string]interface{}{"status": "skipped", "message": "源端已不存在"}).Error
			}
			s.bumpJobProgress(job.ID, int64(len(chunk)), 0, int64(len(repairedIDs)))
		}
	}
	_ = s.systemDB.Model(&models.SyncRepairDiff{}).Where("job_id = ? AND status = ? AND diff_type = ?", diffJobID, "pending", "missing_source").Updates(map[string]interface{}{"status": "skipped", "message": "目标多余数据默认不删除"}).Error
	return nil
}

func (s *RepairService) finishJob(ctx context.Context, job *models.SyncRepairJob, err error, successMessage string) {
	now := time.Now()
	updates := map[string]interface{}{"finished_at": &now}
	if err != nil {
		if ctx.Err() != nil {
			updates["status"] = "canceled"
			updates["message"] = "已取消"
		} else {
			updates["status"] = "failed"
			updates["message"] = "执行失败"
			updates["error_detail"] = err.Error()
		}
	} else {
		updates["status"] = "success"
		updates["message"] = successMessage
		updates["progress_percent"] = 100
	}
	_ = s.systemDB.Model(job).Updates(updates).Error
	_ = NewSyncService().UpdateTask(job.TaskID, map[string]interface{}{"repair_status": "idle"})
}

func (s *RepairService) recordDiff(job *models.SyncRepairJob, mapping *models.SyncTaskTable, sourcePK, targetPK, diffType, sourceHash, targetHash, message string) {
	diff := models.SyncRepairDiff{JobID: job.ID, TaskID: job.TaskID, TaskTableID: mapping.ID, SourceTable: mapping.SourceTable, TargetTable: mapping.TargetTable, SourcePK: sourcePK, TargetPK: targetPK, DiffType: diffType, SourceHash: sourceHash, TargetHash: targetHash, Status: "pending", Message: message}
	_ = s.systemDB.Create(&diff).Error
	s.bumpJobProgress(job.ID, 0, 1, 0)
}

func (s *RepairService) bumpJobProgress(jobID uint, processed, diffs, repaired int64) {
	_ = s.systemDB.Model(&models.SyncRepairJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"processed_rows": gorm.Expr("processed_rows + ?", processed),
		"total_rows":     gorm.Expr("total_rows + ?", processed),
		"diff_rows":      gorm.Expr("diff_rows + ?", diffs),
		"repaired_rows":  gorm.Expr("repaired_rows + ?", repaired),
	}).Error
}

func readRepairRows(db *gorm.DB, table, pk string, columns []string, lastPK, cutoffColumn string, cutoffTime *time.Time) ([]map[string]interface{}, error) {
	selectList := quotedColumns(columns)
	query := "SELECT " + strings.Join(selectList, ",") + " FROM " + quoteMySQL(table)
	params := []interface{}{}
	wheres := []string{}
	if lastPK != "" {
		wheres = append(wheres, quoteMySQL(pk)+" > ?")
		params = append(params, lastPK)
	}
	if cutoffTime != nil && cutoffColumn != "" {
		wheres = append(wheres, quoteMySQL(cutoffColumn)+" <= ?")
		params = append(params, *cutoffTime)
	}
	if len(wheres) > 0 {
		query += " WHERE " + strings.Join(wheres, " AND ")
	}
	query += " ORDER BY " + quoteMySQL(pk) + fmt.Sprintf(" LIMIT %d", repairBatchSize)
	return scanRows(db, query, params...)
}

func readSingleTargetRow(db *gorm.DB, table, pk string, columns []string, pkValue string) (map[string]interface{}, error) {
	rows, err := scanRows(db, "SELECT "+strings.Join(quotedColumns(columns), ",")+" FROM "+quoteMySQL(table)+" WHERE "+quoteMySQL(pk)+" = ? LIMIT 1", pkValue)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func readSingleSourceRow(db *gorm.DB, table, pk string, columns []string, pkValue string) (map[string]interface{}, error) {
	rows, err := scanRows(db, "SELECT "+strings.Join(quotedColumns(columns), ",")+" FROM "+quoteMySQL(table)+" WHERE "+quoteMySQL(pk)+" = ? LIMIT 1", pkValue)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func readSourceRowsByPKs(db *gorm.DB, table, pk string, columns []string, pkValues []string) (map[string]map[string]interface{}, error) {
	if len(pkValues) == 0 {
		return map[string]map[string]interface{}{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(pkValues)), ",")
	args := make([]interface{}, len(pkValues))
	for i := range pkValues {
		args[i] = pkValues[i]
	}
	rows, err := scanRows(db, "SELECT "+strings.Join(quotedColumns(columns), ",")+" FROM "+quoteMySQL(table)+" WHERE "+quoteMySQL(pk)+" IN ("+placeholders+")", args...)
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]interface{}{}
	for _, row := range rows {
		result[valueString(row[pk])] = row
	}
	return result, nil
}

func repairDiffPKs(diffs []models.SyncRepairDiff) []string {
	pks := make([]string, len(diffs))
	for i := range diffs {
		pks[i] = diffs[i].SourcePK
	}
	return pks
}

func sourcePKExists(db *gorm.DB, table, pk, pkValue, cutoffColumn string, cutoffTime *time.Time) (bool, error) {
	query := "SELECT " + quoteMySQL(pk) + " FROM " + quoteMySQL(table) + " WHERE " + quoteMySQL(pk) + " = ?"
	params := []interface{}{pkValue}
	if cutoffTime != nil && cutoffColumn != "" {
		query += " AND " + quoteMySQL(cutoffColumn) + " <= ?"
		params = append(params, *cutoffTime)
	}
	query += " LIMIT 1"
	rows, err := scanRows(db, query, params...)
	return len(rows) > 0, err
}

func scanRows(db *gorm.DB, query string, params ...interface{}) ([]map[string]interface{}, error) {
	rows, err := db.Raw(query, params...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := []map[string]interface{}{}
	for rows.Next() {
		values, pointers := make([]interface{}, len(columns)), make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := map[string]interface{}{}
		for i, column := range columns {
			row[column] = normalizeMySQLScannedValue(values[i])
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func hashRepairRow(row map[string]interface{}, pairs []syncColumnPair, source bool) string {
	values := map[string]interface{}{}
	for _, pair := range pairs {
		key := pair.target
		sourceKey := pair.source
		if source {
			values[key] = normalizeHashValue(row[sourceKey])
		} else {
			values[key] = normalizeHashValue(row[key])
		}
	}
	bytes, _ := json.Marshal(values)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func normalizeHashValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02 15:04:05.999999")
	default:
		return typed
	}
}

func sourcePairColumns(pairs []syncColumnPair) []string {
	columns := make([]string, len(pairs))
	for i := range pairs {
		columns[i] = pairs[i].source
	}
	return columns
}

func targetPairColumns(pairs []syncColumnPair) []string {
	columns := make([]string, len(pairs))
	for i := range pairs {
		columns[i] = pairs[i].target
	}
	return columns
}

func quotedColumns(columns []string) []string {
	quoted := make([]string, len(columns))
	for i, column := range columns {
		quoted[i] = quoteMySQL(column)
	}
	return quoted
}
