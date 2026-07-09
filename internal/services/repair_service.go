package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
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

type RepairDiffView struct {
	models.SyncRepairDiff
	Fields []RepairFieldDiff `json:"fields"`
}

type RepairFieldDiff struct {
	SourceField string      `json:"source_field"`
	TargetField string      `json:"target_field"`
	SourceValue interface{} `json:"source_value"`
	TargetValue interface{} `json:"target_value"`
	Equal       bool        `json:"equal"`
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

func (s *RepairService) ListDiffs(jobID uint, page, pageSize int) ([]RepairDiffView, int64, error) {
	var total int64
	query := s.systemDB.Model(&models.SyncRepairDiff{}).Where("job_id = ?", jobID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var diffs []models.SyncRepairDiff
	err := query.Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&diffs).Error
	if err != nil {
		return nil, 0, err
	}
	views, err := s.enrichDiffs(diffs)
	return views, total, err
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
	sourceAllColumns, err := mysqlColumnNamesFromDB(sourceDB, mapping.SourceTable)
	if err != nil {
		return err
	}
	cutoffColumn := ""
	if job.CutoffTime != nil && job.CutoffColumn != "" && containsString(sourceAllColumns, job.CutoffColumn) {
		cutoffColumn = job.CutoffColumn
	}
	sourceTotal, err := countRepairRows(sourceDB, mapping.SourceTable, cutoffColumn, job.CutoffTime)
	if err != nil {
		return err
	}
	targetTotal, err := countRepairRows(targetDB, mapping.TargetTable, "", nil)
	if err != nil {
		return err
	}
	s.addJobTotal(job.ID, sourceTotal+targetTotal)
	lastPK := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := readRepairRows(sourceDB, mapping.SourceTable, mapping.SourcePrimaryKey, sourcePairColumns(pairs), lastPK, cutoffColumn, job.CutoffTime)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		targetRows, err := readRowsByPKs(targetDB, mapping.TargetTable, mapping.TargetPrimaryKey, targetPairColumns(pairs), repairRowPKs(rows, mapping.SourcePrimaryKey))
		if err != nil {
			return err
		}
		diffs := make([]models.SyncRepairDiff, 0)
		for _, row := range rows {
			lastPK = valueString(row[mapping.SourcePrimaryKey])
			sourceHash := hashRepairRow(row, pairs, true)
			targetRow := targetRows[lastPK]
			if targetRow == nil {
				diffs = append(diffs, newRepairDiff(job, mapping, lastPK, lastPK, "missing_target", sourceHash, "", "目标缺少数据"))
			} else {
				targetHash := hashRepairRow(targetRow, pairs, false)
				if sourceHash != targetHash {
					diffs = append(diffs, newRepairDiff(job, mapping, lastPK, lastPK, "mismatch", sourceHash, targetHash, mismatchMessage(row, targetRow, pairs)))
				}
			}
		}
		if err := s.recordDiffs(job.ID, diffs); err != nil {
			return err
		}
		s.bumpJobProgress(job.ID, int64(len(rows)), 0, 0)
	}
	return s.compareTargetExtras(ctx, job, mapping, sourceDB, targetDB, pairs, cutoffColumn)
}

func (s *RepairService) compareTargetExtras(ctx context.Context, job *models.SyncRepairJob, mapping *models.SyncTaskTable, sourceDB, targetDB *gorm.DB, pairs []syncColumnPair, cutoffColumn string) error {
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
		sourceRows, err := readRowsByPKsWithCutoff(sourceDB, mapping.SourceTable, mapping.SourcePrimaryKey, []string{mapping.SourcePrimaryKey}, repairRowPKs(rows, mapping.TargetPrimaryKey), cutoffColumn, job.CutoffTime)
		if err != nil {
			return err
		}
		diffs := make([]models.SyncRepairDiff, 0)
		for _, row := range rows {
			lastPK = valueString(row[mapping.TargetPrimaryKey])
			if sourceRows[lastPK] == nil {
				diffs = append(diffs, newRepairDiff(job, mapping, lastPK, lastPK, "missing_source", "", hashRepairRow(row, pairs, false), "源端缺少数据"))
			}
		}
		if err := s.recordDiffs(job.ID, diffs); err != nil {
			return err
		}
		s.bumpJobProgress(job.ID, int64(len(rows)), 0, 0)
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
	s.addJobTotal(job.ID, int64(len(diffs)))
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

func (s *RepairService) enrichDiffs(diffs []models.SyncRepairDiff) ([]RepairDiffView, error) {
	views := make([]RepairDiffView, 0, len(diffs))
	if len(diffs) == 0 {
		return views, nil
	}
	task, err := NewSyncService().GetTask(diffs[0].TaskID)
	if err != nil {
		return nil, err
	}
	sourceDB, err := database.GetManager().GetConnection(task.SourceDB)
	if err != nil {
		return nil, err
	}
	targetDB, err := database.GetManager().GetConnection(task.TargetDB)
	if err != nil {
		return nil, err
	}
	tableByID := map[uint]*models.SyncTaskTable{}
	for i := range task.TaskTables {
		tableByID[task.TaskTables[i].ID] = &task.TaskTables[i]
	}
	for _, diff := range diffs {
		view := RepairDiffView{SyncRepairDiff: diff}
		mapping := tableByID[diff.TaskTableID]
		if mapping == nil {
			views = append(views, view)
			continue
		}
		sourceColumns, err := selectableSourceColumns(task, mapping, sourceDB)
		if err != nil {
			return nil, err
		}
		pairs, err := syncColumnPairs(targetDB, mapping, sourceColumns)
		if err != nil {
			return nil, err
		}
		sourceRow, err := readSingleSourceRow(sourceDB, mapping.SourceTable, mapping.SourcePrimaryKey, sourcePairColumns(pairs), diff.SourcePK)
		if err != nil {
			return nil, err
		}
		targetRow, err := readSingleSourceRow(targetDB, mapping.TargetTable, mapping.TargetPrimaryKey, targetPairColumns(pairs), diff.TargetPK)
		if err != nil {
			return nil, err
		}
		view.Fields = compareRepairFields(sourceRow, targetRow, pairs)
		views = append(views, view)
	}
	return views, nil
}

func newRepairDiff(job *models.SyncRepairJob, mapping *models.SyncTaskTable, sourcePK, targetPK, diffType, sourceHash, targetHash, message string) models.SyncRepairDiff {
	return models.SyncRepairDiff{JobID: job.ID, TaskID: job.TaskID, TaskTableID: mapping.ID, SourceTable: mapping.SourceTable, TargetTable: mapping.TargetTable, SourcePK: sourcePK, TargetPK: targetPK, DiffType: diffType, SourceHash: sourceHash, TargetHash: targetHash, Status: "pending", Message: message}
}

func (s *RepairService) recordDiffs(jobID uint, diffs []models.SyncRepairDiff) error {
	if len(diffs) == 0 {
		return nil
	}
	if err := s.systemDB.CreateInBatches(diffs, 500).Error; err != nil {
		return err
	}
	s.bumpJobProgress(jobID, 0, int64(len(diffs)), 0)
	return nil
}

func (s *RepairService) addJobTotal(jobID uint, total int64) {
	if total <= 0 {
		return
	}
	_ = s.systemDB.Model(&models.SyncRepairJob{}).Where("id = ?", jobID).Update("total_rows", gorm.Expr("total_rows + ?", total)).Error
}

func (s *RepairService) bumpJobProgress(jobID uint, processed, diffs, repaired int64) {
	_ = s.systemDB.Model(&models.SyncRepairJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"processed_rows":   gorm.Expr("processed_rows + ?", processed),
		"diff_rows":        gorm.Expr("diff_rows + ?", diffs),
		"repaired_rows":    gorm.Expr("repaired_rows + ?", repaired),
		"progress_percent": gorm.Expr("CASE WHEN total_rows > 0 AND (processed_rows + ?) * 100.0 / total_rows > 99.9 THEN 99.9 WHEN total_rows > 0 THEN (processed_rows + ?) * 100.0 / total_rows ELSE progress_percent END", processed, processed),
	}).Error
}

func countRepairRows(db *gorm.DB, table, cutoffColumn string, cutoffTime *time.Time) (int64, error) {
	query := "SELECT COUNT(*) AS cnt FROM " + quoteMySQL(table)
	params := []interface{}{}
	if cutoffTime != nil && cutoffColumn != "" {
		query += " WHERE " + quoteMySQL(cutoffColumn) + " <= ?"
		params = append(params, *cutoffTime)
	}
	var count int64
	if err := db.Raw(query, params...).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
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

func readSingleSourceRow(db *gorm.DB, table, pk string, columns []string, pkValue string) (map[string]interface{}, error) {
	rows, err := scanRows(db, "SELECT "+strings.Join(quotedColumns(columns), ",")+" FROM "+quoteMySQL(table)+" WHERE "+quoteMySQL(pk)+" = ? LIMIT 1", pkValue)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func readSourceRowsByPKs(db *gorm.DB, table, pk string, columns []string, pkValues []string) (map[string]map[string]interface{}, error) {
	return readRowsByPKs(db, table, pk, columns, pkValues)
}

func readRowsByPKs(db *gorm.DB, table, pk string, columns []string, pkValues []string) (map[string]map[string]interface{}, error) {
	return readRowsByPKsWithCutoff(db, table, pk, columns, pkValues, "", nil)
}

func readRowsByPKsWithCutoff(db *gorm.DB, table, pk string, columns []string, pkValues []string, cutoffColumn string, cutoffTime *time.Time) (map[string]map[string]interface{}, error) {
	if len(pkValues) == 0 {
		return map[string]map[string]interface{}{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(pkValues)), ",")
	args := make([]interface{}, len(pkValues))
	for i := range pkValues {
		args[i] = pkValues[i]
	}
	query := "SELECT " + strings.Join(quotedColumns(columns), ",") + " FROM " + quoteMySQL(table) + " WHERE " + quoteMySQL(pk) + " IN (" + placeholders + ")"
	if cutoffTime != nil && cutoffColumn != "" {
		query += " AND " + quoteMySQL(cutoffColumn) + " <= ?"
		args = append(args, *cutoffTime)
	}
	rows, err := scanRows(db, query, args...)
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]interface{}{}
	for _, row := range rows {
		result[valueString(row[pk])] = row
	}
	return result, nil
}

func repairRowPKs(rows []map[string]interface{}, pk string) []string {
	pks := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		value := valueString(row[pk])
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		pks = append(pks, value)
	}
	return pks
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
			values[key] = comparableValue(row[sourceKey])
		} else {
			values[key] = comparableValue(row[key])
		}
	}
	bytes, _ := json.Marshal(values)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func mismatchMessage(sourceRow, targetRow map[string]interface{}, pairs []syncColumnPair) string {
	fields := []string{}
	for _, field := range compareRepairFields(sourceRow, targetRow, pairs) {
		if !field.Equal {
			fields = append(fields, field.TargetField)
		}
	}
	if len(fields) == 0 {
		return "字段值不一致"
	}
	if len(fields) > 8 {
		fields = append(fields[:8], fmt.Sprintf("等 %d 个字段", len(fields)))
	}
	return "字段值不一致: " + strings.Join(fields, ", ")
}

func compareRepairFields(sourceRow, targetRow map[string]interface{}, pairs []syncColumnPair) []RepairFieldDiff {
	fields := make([]RepairFieldDiff, 0, len(pairs))
	for _, pair := range pairs {
		sourceValue := valueFromRow(sourceRow, pair.source)
		targetValue := valueFromRow(targetRow, pair.target)
		fields = append(fields, RepairFieldDiff{
			SourceField: pair.source,
			TargetField: pair.target,
			SourceValue: displayRepairValue(sourceValue),
			TargetValue: displayRepairValue(targetValue),
			Equal:       comparableValue(sourceValue) == comparableValue(targetValue),
		})
	}
	return fields
}

func valueFromRow(row map[string]interface{}, key string) interface{} {
	if row == nil {
		return nil
	}
	return row[key]
}

func displayRepairValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02 15:04:05.999999")
	case []byte:
		return string(typed)
	default:
		return typed
	}
}

func comparableValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return "<NULL>"
	case time.Time:
		return normalizeTimeComparable(typed)
	case []byte:
		return comparableString(string(typed))
	case string:
		return comparableString(typed)
	case bool:
		if typed {
			return "1"
		}
		return "0"
	case int:
		return strconv.FormatInt(int64(typed), 10)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case float64:
		if math.IsNaN(typed) {
			return "NaN"
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return fmt.Sprint(typed)
	}
}

func comparableString(value string) string {
	if parsed, ok := parseComparableTime(value); ok {
		return normalizeTimeComparable(parsed)
	}
	return value
}

func parseComparableTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05.999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func normalizeTimeComparable(value time.Time) string {
	return value.Format("2006-01-02 15:04:05.999999")
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
